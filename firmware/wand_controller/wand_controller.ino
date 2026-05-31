#include <CodeCell.h>
#include <WiFiManager.h>  // pulls in WiFi.h; provides the captive-portal provisioning
#include <WiFiUdp.h>

// --- Configuration ---
// Create config.h from config.h.example. WiFi credentials are OPTIONAL there
// now — see the provisioning note below.
#include "config.h"

// WiFi credentials are optional — default to empty if config.h omits (or
// comments out) them, so the sketch always builds. Empty means "provision via
// the setup portal".
#ifndef WIFI_SSID
#define WIFI_SSID ""
#endif
#ifndef WIFI_PASSWORD
#define WIFI_PASSWORD ""
#endif

const int UDP_PORT = 9999;

// --- WiFi provisioning ---
// On first boot — or whenever saved credentials fail — the wand opens its own
// open WiFi network ("Toy Box Wand Setup"). A parent joins it on a phone and a
// captive page lets them pick their home WiFi: no recompile, no hardcoded
// credentials. config.h's WIFI_SSID, if set, only seeds a never-provisioned
// wand (dev/CI convenience); a portal-provisioned network always wins after.
const char *SETUP_AP_NAME              = "Toy Box Wand Setup";
const unsigned long WIFI_CONNECT_TIMEOUT_S = 20;      // try saved creds this long before opening the portal
const unsigned long WIFI_PORTAL_TIMEOUT_S  = 180;     // close an unattended portal and reboot to retry
const unsigned long WIFI_LOST_RESTART_MS   = 120000;  // reboot after this long offline so a router change re-opens the portal

// Broadcast address for discovery
IPAddress broadcastIP(255, 255, 255, 255);

// --- Protocol ---
const uint8_t MAGIC0   = 0x57; // 'W'
const uint8_t MAGIC1   = 0x44; // 'D'
const uint8_t VERSION  = 0x02;
const int PACKET_SIZE  = 44;
const int CTRL_SIZE    = 4;

// Control packet types
const uint8_t TYPE_DISCOVERY = 0x01;
const uint8_t TYPE_ACK       = 0x02;

// Discovery/connection timing
const unsigned long DISCOVERY_INTERVAL_MS = 500;
const unsigned long ACK_TIMEOUT_MS        = 5000;
const unsigned int  MAX_SEND_FAILURES     = 10;

// --- State machine ---
enum WandState { DISCOVERING, STREAMING };
WandState state = DISCOVERING;
IPAddress listenerIP;
unsigned long lastAckTime = 0;
unsigned long lastDiscoveryTime = 0;
unsigned int  sendFailures = 0;
unsigned long wifiLostSince = 0;

CodeCell myCodeCell;
WiFiUDP udp;
uint8_t seq = 0;

// Rotate a quaternion from sensor frame into wand body frame via
// r * q * r^-1 (conjugation). r is WAND_REMAP from config.h; identity by
// default, calibrated by holding the wand at its neutral pose and setting
// r to the inverse of the sensor-frame quaternion read at that pose.
static inline void apply_wand_remap(float &qw, float &qx, float &qy, float &qz) {
  const float rw = WAND_REMAP_W, rx = WAND_REMAP_X, ry = WAND_REMAP_Y, rz = WAND_REMAP_Z;

  // t = r * q
  float tw = rw*qw - rx*qx - ry*qy - rz*qz;
  float tx = rw*qx + rx*qw + ry*qz - rz*qy;
  float ty = rw*qy - rx*qz + ry*qw + rz*qx;
  float tz = rw*qz + rx*qy - ry*qx + rz*qw;

  // out = t * r^-1 (conjugate of unit r)
  qw = tw*rw - tx*(-rx) - ty*(-ry) - tz*(-rz);
  qx = tw*(-rx) + tx*rw + ty*(-rz) - tz*(-ry);
  qy = tw*(-ry) - tx*(-rz) + ty*rw + tz*(-rx);
  qz = tw*(-rz) + tx*(-ry) - ty*(-rx) + tz*rw;
}

// Drive the onboard RGB LED so a parent can see the wand's state during setup.
// (CodeCell::Run() reclaims the LED for battery status during normal play.)
static void setLED(uint8_t r, uint8_t g, uint8_t b) {
  myCodeCell.LED(r, g, b);
}

// Bring up WiFi. WiFiManager owns the saved credentials: it reconnects to the
// last network a parent set up, and only opens the "Toy Box Wand Setup" portal
// when that fails. config.h's WIFI_SSID, if set, seeds ONLY a never-provisioned
// wand (a dev/CI convenience), so a portal-provisioned network always wins on
// later boots. Blocks until connected; reboots to retry if the portal is left
// unattended.
void connectWiFi() {
  setLED(0, 0, 40);  // dim blue: working on it

  // Initialize the WiFi driver so any saved credentials are loaded from NVS
  // before we inspect them (getWiFiIsSaved reads the stored station config).
  WiFi.mode(WIFI_STA);

  WiFiManager wm;
  wm.setConnectTimeout(WIFI_CONNECT_TIMEOUT_S);
  wm.setConfigPortalTimeout(WIFI_PORTAL_TIMEOUT_S);
  wm.setAPCallback([](WiFiManager *) {
    setLED(0, 0, 255);  // solid blue: "join my WiFi to set me up"
    Serial.print("Setup portal open — join WiFi network: ");
    Serial.println(SETUP_AP_NAME);
  });

  // Seed from config.h only on a never-provisioned wand. Once any network is
  // saved (here or via the portal) it wins on every boot and config.h is
  // ignored until the flash is erased. Gating on !getWiFiIsSaved() keeps the
  // preloaded default from ever overriding a real saved network.
  if (strlen(WIFI_SSID) > 0 && !wm.getWiFiIsSaved()) {
    Serial.print("No saved WiFi — seeding from config.h: ");
    Serial.println(WIFI_SSID);
    wm.preloadWiFi(WIFI_SSID, WIFI_PASSWORD);
  }

  // Reconnect with saved creds (or the config.h seed); open the
  // "Toy Box Wand Setup" portal (open network, no password) only if that fails.
  bool connected = wm.autoConnect(SETUP_AP_NAME);

  if (!connected) {
    Serial.println("WiFi setup timed out — restarting to try again.");
    delay(500);
    ESP.restart();
  }

  setLED(0, 255, 0);  // brief green: connected
  Serial.print("Connected! IP: ");
  Serial.println(WiFi.localIP());
}

void setup() {
  Serial.begin(115200);

  // Game rotation vector (no-mag quaternion) + linear acceleration (gravity
  // removed) + raw gyro.
  myCodeCell.Init(MOTION_ROTATION_NO_MAG + MOTION_LINEAR_ACC + MOTION_GYRO);

  connectWiFi();

  udp.begin(UDP_PORT);
}

// Drain all pending packets, returning true if any ack was found.
bool checkForAck() {
  bool foundAck = false;
  int packetSize;
  while ((packetSize = udp.parsePacket()) > 0) {
    if (packetSize >= CTRL_SIZE) {
      uint8_t inBuf[CTRL_SIZE];
      udp.read(inBuf, CTRL_SIZE);
      if (inBuf[0] == MAGIC0 && inBuf[1] == MAGIC1 &&
          inBuf[2] == VERSION && inBuf[3] == TYPE_ACK) {
        listenerIP = udp.remoteIP();
        lastAckTime = millis();
        foundAck = true;
      }
    }
  }
  return foundAck;
}

// Reinitialize UDP and enter discovery state.
void enterDiscovering() {
  state = DISCOVERING;
  sendFailures = 0;
  udp.stop();
  udp.begin(UDP_PORT);
  Serial.println("Rediscovering...");
}

void sendDiscovery() {
  uint8_t pkt[CTRL_SIZE] = { MAGIC0, MAGIC1, VERSION, TYPE_DISCOVERY };
  udp.beginPacket(broadcastIP, UDP_PORT);
  udp.write(pkt, CTRL_SIZE);
  udp.endPacket();
}

void loop() {
  if (myCodeCell.Run(40)) {

    // Reconnect WiFi if dropped. If we stay offline a long time (e.g. the
    // household router or its password changed), reboot so setup() can
    // re-open the provisioning portal.
    if (WiFi.status() != WL_CONNECTED) {
      if (wifiLostSince == 0) {
        wifiLostSince = millis();
      } else if (millis() - wifiLostSince > WIFI_LOST_RESTART_MS) {
        Serial.println("WiFi lost too long — restarting to re-provision.");
        ESP.restart();
      }
      WiFi.reconnect();
      return;
    }
    wifiLostSince = 0;

    switch (state) {
      case DISCOVERING: {
        unsigned long now = millis();
        if (now - lastDiscoveryTime >= DISCOVERY_INTERVAL_MS) {
          sendDiscovery();
          lastDiscoveryTime = now;
          Serial.println("Discovering listener...");
        }
        if (checkForAck()) {
          state = STREAMING;
          Serial.print("Listener found: ");
          Serial.println(listenerIP);
        }
        break;
      }

      case STREAMING: {
        // Check for keepalive acks
        checkForAck();

        // Timeout — fall back to discovery
        if (millis() - lastAckTime > ACK_TIMEOUT_MS) {
          enterDiscovering();
          break;
        }

        // Read IMU data: game rotation quaternion + linear accel + gyro
        float qw, qx, qy, qz;
        myCodeCell.Motion_RotationNoMagVectorRead(qw, qx, qy, qz);
        apply_wand_remap(qw, qx, qy, qz);

        float ax, ay, az;
        myCodeCell.Motion_LinearAccRead(ax, ay, az);

        float gx, gy, gz;
        myCodeCell.Motion_GyroRead(gx, gy, gz);

        // Build 44-byte data packet
        uint8_t packet[PACKET_SIZE];
        packet[0] = MAGIC0;
        packet[1] = MAGIC1;
        packet[2] = VERSION;
        packet[3] = seq++;

        // ESP32-C6 is little-endian, matching our protocol
        memcpy(&packet[4],  &qw, 4);
        memcpy(&packet[8],  &qx, 4);
        memcpy(&packet[12], &qy, 4);
        memcpy(&packet[16], &qz, 4);
        memcpy(&packet[20], &ax, 4);
        memcpy(&packet[24], &ay, 4);
        memcpy(&packet[28], &az, 4);
        memcpy(&packet[32], &gx, 4);
        memcpy(&packet[36], &gy, 4);
        memcpy(&packet[40], &gz, 4);

        // Send via unicast UDP — track failures to detect dead host
        int ok = udp.beginPacket(listenerIP, UDP_PORT);
        if (ok) {
          udp.write(packet, PACKET_SIZE);
          ok = udp.endPacket();
        }
        if (ok) {
          sendFailures = 0;
        } else if (++sendFailures >= MAX_SEND_FAILURES) {
          enterDiscovering();
        }
        break;
      }
    }
  }
}
