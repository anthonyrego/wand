#include <CodeCell.h>
#include <WiFi.h>
#include <WiFiUdp.h>

// --- Configuration ---
// Create config.h from config.h.example and set your WiFi credentials.
#include "config.h"
const int UDP_PORT = 9999;

// Broadcast address for discovery
IPAddress broadcastIP(255, 255, 255, 255);

// --- Protocol ---
const uint8_t MAGIC0   = 0x57; // 'W'
const uint8_t MAGIC1   = 0x44; // 'D'
const uint8_t VERSION  = 0x01;
const int PACKET_SIZE  = 40;
const int CTRL_SIZE    = 4;

// Control packet types
const uint8_t TYPE_DISCOVERY = 0x01;
const uint8_t TYPE_ACK       = 0x02;

// Discovery/connection timing
const unsigned long DISCOVERY_INTERVAL_MS = 500;
const unsigned long ACK_TIMEOUT_MS        = 5000;

// --- State machine ---
enum WandState { DISCOVERING, STREAMING };
WandState state = DISCOVERING;
IPAddress listenerIP;
unsigned long lastAckTime = 0;
unsigned long lastDiscoveryTime = 0;

CodeCell myCodeCell;
WiFiUDP udp;
uint8_t seq = 0;

void setup() {
  Serial.begin(115200);

  // Initialize CodeCell with rotation + accelerometer + gyro
  myCodeCell.Init(MOTION_ROTATION_NO_MAG + MOTION_ACCELEROMETER + MOTION_GYRO);

  // Connect to WiFi
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.print("Connecting to WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println();
  Serial.print("Connected! IP: ");
  Serial.println(WiFi.localIP());

  udp.begin(UDP_PORT);
}

// Check for incoming ack packets. Returns true if an ack was received.
bool checkForAck() {
  int packetSize = udp.parsePacket();
  if (packetSize >= CTRL_SIZE) {
    uint8_t inBuf[CTRL_SIZE];
    udp.read(inBuf, CTRL_SIZE);
    if (inBuf[0] == MAGIC0 && inBuf[1] == MAGIC1 &&
        inBuf[2] == VERSION && inBuf[3] == TYPE_ACK) {
      listenerIP = udp.remoteIP();
      lastAckTime = millis();
      return true;
    }
  }
  return false;
}

void sendDiscovery() {
  uint8_t pkt[CTRL_SIZE] = { MAGIC0, MAGIC1, VERSION, TYPE_DISCOVERY };
  udp.beginPacket(broadcastIP, UDP_PORT);
  udp.write(pkt, CTRL_SIZE);
  udp.endPacket();
}

void loop() {
  if (myCodeCell.Run(60)) {

    // Reconnect WiFi if dropped
    if (WiFi.status() != WL_CONNECTED) {
      WiFi.reconnect();
      return;
    }

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
          state = DISCOVERING;
          Serial.println("Listener lost, rediscovering...");
          break;
        }

        // Read IMU data
        float roll, pitch, yaw;
        myCodeCell.Motion_RotationNoMagRead(roll, pitch, yaw);

        float ax, ay, az;
        myCodeCell.Motion_AccelerometerRead(ax, ay, az);

        float gx, gy, gz;
        myCodeCell.Motion_GyroRead(gx, gy, gz);

        // Build 40-byte data packet
        uint8_t packet[PACKET_SIZE];
        packet[0] = MAGIC0;
        packet[1] = MAGIC1;
        packet[2] = VERSION;
        packet[3] = seq++;

        // ESP32-C6 is little-endian, matching our protocol
        memcpy(&packet[4],  &roll,  4);
        memcpy(&packet[8],  &pitch, 4);
        memcpy(&packet[12], &yaw,   4);
        memcpy(&packet[16], &ax,    4);
        memcpy(&packet[20], &ay,    4);
        memcpy(&packet[24], &az,    4);
        memcpy(&packet[28], &gx,    4);
        memcpy(&packet[32], &gy,    4);
        memcpy(&packet[36], &gz,    4);

        // Send via unicast UDP to discovered listener
        udp.beginPacket(listenerIP, UDP_PORT);
        udp.write(packet, PACKET_SIZE);
        udp.endPacket();
        break;
      }
    }
  }
}
