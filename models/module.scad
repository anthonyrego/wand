// Core Module — Cylindrical puck containing CodeCell C6 + LiPo battery
// Self-contained, USB-C accessible, bayonet pins for housing mount.
// Open in OpenSCAD: F5 to preview, F6 to render, export STL for printing.

include <mount.scad>

/* ── Module parameters ────────────────────────────────── */

// Outer dimensions
module_d           = 30;      // outer diameter (mm)
module_h           = 18;      // total height (mm)
module_fillet      = 2.0;     // edge rounding radius (mm)
module_wall        = 2.0;     // minimum wall thickness (mm)

// Internal cavity
cavity_d           = 26;      // inner cavity diameter (module_d - 2*wall)
cavity_h           = 14;      // inner cavity height (module_h - 2*wall)

// Battery compartment (bottom of cavity)
battery_h          = 5;       // LiPo cell thickness
battery_w          = 20;      // battery width (flat pouch cell)
battery_l          = 22;      // battery length

// PCB (CodeCell C6, sits above battery)
pcb_w              = 25;      // PCB width (CodeCell is ~25mm)
pcb_l              = 25;      // PCB length
pcb_thickness      = 1.6;     // PCB board thickness
pcb_standoff_h     = 2.0;     // standoff height above battery shelf
pcb_standoff_d     = 4.0;     // standoff outer diameter
pcb_standoff_hole  = 1.6;     // M1.6 screw hole
pcb_mount_spacing  = 20;      // between mounting holes

// Shell assembly — 3 screw bosses joining top + bottom halves
shell_screw_d      = 1.8;     // M2 heat-set insert hole
shell_boss_d       = 5.0;     // boss outer diameter
shell_screw_count  = 3;       // 3 screws at 120 degrees

// Split plane — where top and bottom halves meet
split_z            = module_wall + battery_h + pcb_standoff_h;
// This puts the split just above the battery, below the PCB.
// Bottom half holds battery + standoffs; top half caps the PCB.

// USB-C port (on the side wall, near the bottom)
usbc_width         = 9.5;
usbc_height        = 3.5;
usbc_z             = module_wall + 0.5;  // just above interior floor
usbc_depth         = module_wall + 1;    // cuts through wall

// Bayonet pins — positioned on the lower portion of the cylinder
bayonet_z          = 1.0;     // z offset from bottom of module

// Display options
show_cross_section = false;
show_assembly      = true;
show_exploded      = false;
show_top           = true;
show_bottom        = true;
explode_distance   = 12;

$fn = 80;

/* ── Modules ──────────────────────────────────────────── */

// Filleted cylinder — the basic outer shell shape
module filleted_cylinder(d, h, fillet) {
    r = d / 2;
    hull() {
        // Bottom fillet torus
        translate([0, 0, fillet])
            rotate_extrude()
                translate([r - fillet, 0, 0])
                    circle(r = fillet);
        // Top fillet torus
        translate([0, 0, h - fillet])
            rotate_extrude()
                translate([r - fillet, 0, 0])
                    circle(r = fillet);
    }
}

// Internal cavity (cylindrical bore)
module internal_cavity() {
    translate([0, 0, module_wall])
        cylinder(d = cavity_d, h = cavity_h);
}

// Battery pocket — a rectangular recess in the bottom of the cavity
module battery_pocket() {
    translate([-battery_w/2, -battery_l/2, module_wall])
        cube([battery_w, battery_l, battery_h]);
}

// PCB standoffs — 4 posts rising from the battery shelf
module pcb_standoffs() {
    s = pcb_mount_spacing / 2;
    positions = [
        [ s,  s],
        [-s,  s],
        [-s, -s],
        [ s, -s]
    ];
    for (p = positions) {
        translate([p[0], p[1], module_wall + battery_h]) {
            difference() {
                cylinder(d = pcb_standoff_d, h = pcb_standoff_h);
                translate([0, 0, -0.1])
                    cylinder(d = pcb_standoff_hole, h = pcb_standoff_h + 0.2);
            }
        }
    }
}

// Shell screw boss positions — 3 at 120 degrees, near the outer wall
function shell_boss_positions() = [
    for (i = [0 : shell_screw_count - 1])
        let(angle = i * 360 / shell_screw_count + 30)  // +30 to avoid USB-C
        [(cavity_d/2 - shell_boss_d/2) * cos(angle),
         (cavity_d/2 - shell_boss_d/2) * sin(angle)]
];

// Screw bosses (solid cylinders bridging the split)
module shell_bosses(from_z, height) {
    for (p = shell_boss_positions()) {
        translate([p[0], p[1], from_z])
            cylinder(d = shell_boss_d, h = height);
    }
}

// Screw holes through bosses
module shell_screw_holes() {
    for (p = shell_boss_positions()) {
        translate([p[0], p[1], -1])
            cylinder(d = shell_screw_d, h = module_h + 2);
    }
}

// USB-C port cutout — on the side wall
module usbc_cutout() {
    translate([-usbc_width/2, module_d/2 - usbc_depth, usbc_z])
        cube([usbc_width, usbc_depth + 1, usbc_height]);
}

/* ── Bottom half ──────────────────────────────────────── */

module module_bottom() {
    color("dimgray", 0.9)
    difference() {
        union() {
            // Outer shell — bottom portion
            intersection() {
                filleted_cylinder(module_d, module_h, module_fillet);
                cylinder(d = module_d + 2, h = split_z);
            }
            // Screw bosses rising into the cavity
            shell_bosses(module_wall, split_z - module_wall);
            // Bayonet pins on outer wall
            mount_male(module_d / 2, bayonet_z);
        }
        // Internal cavity
        internal_cavity();
        // Screw holes
        shell_screw_holes();
        // USB-C cutout
        usbc_cutout();
    }

    // PCB standoffs (added after difference so they don't get carved)
    color("dimgray", 0.9)
        pcb_standoffs();
}

/* ── Top half ─────────────────────────────────────────── */

module module_top() {
    color("slategray", 0.9)
    difference() {
        union() {
            // Outer shell — top portion
            intersection() {
                filleted_cylinder(module_d, module_h, module_fillet);
                translate([0, 0, split_z])
                    cylinder(d = module_d + 2, h = module_h - split_z);
            }
            // Screw bosses dropping into the cavity
            shell_bosses(split_z, module_h - module_wall - split_z);
        }
        // Internal cavity
        internal_cavity();
        // Screw holes
        shell_screw_holes();
    }
}

/* ── Assembly ─────────────────────────────────────────── */

module module_assembly() {
    if (show_exploded) {
        if (show_bottom) module_bottom();
        if (show_top)
            translate([0, 0, explode_distance]) module_top();
    } else {
        if (show_bottom) module_bottom();
        if (show_top) module_top();
    }
}

/* ── Render ────────────────────────────────────────────── */

if (show_cross_section) {
    difference() {
        module_assembly();
        // Cut away front half
        translate([0, -module_d, -1])
            cube([module_d, module_d * 2, module_h + 2]);
    }
} else {
    module_assembly();
}
