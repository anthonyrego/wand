// Bayonet Twist-Lock Mount Interface — shared library
// Used by module.scad (male/pins) and housing_*.scad (female/socket).
// include <mount.scad> in consumer files.

/* ── Mount parameters ─────────────────────────────────── */

// Module outer dimensions (defines the mating interface)
mount_module_d     = 30;      // module outer diameter (mm)
mount_socket_d     = 30.4;    // socket inner diameter (clearance fit)
mount_socket_depth = 10;      // how deep the module sits in the housing

// Bayonet pins (male, on module)
mount_pin_w        = 2.5;     // pin width (mm)
mount_pin_h        = 2.0;     // pin height (mm)
mount_pin_d        = 1.5;     // radial protrusion (mm)
mount_pin_count    = 2;       // always 2, at 180 degrees

// Orientation key — pin A is wider than pin B so only one insertion angle works
mount_key_pin_w    = 3.5;     // keyed pin width (vs 2.5 for the other)

// Bayonet slot geometry (female, in housing)
mount_slot_arc     = 60;      // degrees of twist to lock
mount_slot_clearance = 0.3;   // extra clearance around pins in slot

// Detent (click-feel)
mount_detent_r     = 0.4;     // bump radius on module pin
mount_detent_pocket_r = 0.55; // pocket radius in housing (slightly larger)

// USB-C access channel
mount_usbc_width   = 9.5;     // USB-C plug width + clearance
mount_usbc_height  = 3.5;     // USB-C plug height + clearance
mount_usbc_depth   = 8;       // how deep the cutout goes into housing wall

$fn = 80;

/* ── Male interface (on module) ───────────────────────── */

// Single bayonet pin at the origin, extending radially outward at radius r
module mount_pin(r, width, height, depth) {
    translate([r - depth/2, -width/2, 0])
        cube([depth, width, height]);
}

// Both bayonet pins on a cylinder of given radius, with orientation keying
module mount_pins(r) {
    // Pin A — keyed (wider)
    rotate([0, 0, 0])
        mount_pin(r, mount_key_pin_w, mount_pin_h, mount_pin_d);

    // Pin B — standard
    rotate([0, 0, 180])
        mount_pin(r, mount_pin_w, mount_pin_h, mount_pin_d);
}

// Detent bumps on the module pins (at the trailing edge after twist)
module mount_detent_bumps(r) {
    for (rot = [0, 180]) {
        w = (rot == 0) ? mount_key_pin_w : mount_pin_w;
        rotate([0, 0, rot + mount_slot_arc])
            translate([r, 0, mount_pin_h / 2])
                sphere(r = mount_detent_r);
    }
}

// Complete male bayonet interface — call this from module.scad
// Places pins and detents on the outer wall at the given z offset.
// r = outer radius of the module body
module mount_male(r, z_offset = 0) {
    translate([0, 0, z_offset]) {
        mount_pins(r);
        mount_detent_bumps(r);
    }
}

/* ── Female interface (in housing) ────────────────────── */

// Bayonet socket — cylindrical bore + L-shaped slots + detent pockets
// Subtract this from the housing body.
// socket_extra_depth: extra depth beyond mount_socket_depth (for through-hole)
module mount_socket(socket_extra_depth = 0) {
    r = mount_socket_d / 2;
    total_depth = mount_socket_depth + socket_extra_depth;
    pin_clearance = mount_slot_clearance;

    // Main cylindrical bore
    translate([0, 0, -0.01])
        cylinder(d = mount_socket_d, h = total_depth + 0.01);

    // L-shaped slots for each pin (two pins, 180 apart)
    for (rot = [0, 180]) {
        w = (rot == 0) ? mount_key_pin_w : mount_pin_w;
        slot_w = w + pin_clearance * 2;
        slot_h = mount_pin_h + pin_clearance;
        slot_d = mount_pin_d + pin_clearance + 1; // +1 to break through wall

        rotate([0, 0, rot]) {
            // Vertical entry slot (pin drops in)
            translate([r - mount_pin_d, -slot_w/2, -0.01])
                cube([slot_d, slot_w, slot_h + 0.5]);

            // Horizontal twist slot (pin slides around)
            for (a = [0 : 2 : mount_slot_arc]) {
                rotate([0, 0, a])
                    translate([r - mount_pin_d, -slot_w/2, -0.01])
                        cube([slot_d, slot_w, slot_h]);
            }

            // Detent pocket at end of twist
            rotate([0, 0, mount_slot_arc])
                translate([r, 0, mount_pin_h / 2])
                    sphere(r = mount_detent_pocket_r);
        }
    }
}

// USB-C access channel — a groove from the socket bore to the housing exterior
// Cut this from the housing so the USB-C port stays accessible.
// wall_thickness: how thick the housing wall is where the channel exits
module mount_usbc_channel(wall_thickness) {
    channel_length = mount_socket_d / 2 + wall_thickness + 1;
    translate([-mount_usbc_width/2, -channel_length, -0.01])
        cube([mount_usbc_width, channel_length, mount_usbc_height]);
}
