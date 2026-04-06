// Wand Enclosure — Star head + handle with bayonet twist-lock
// Open in OpenSCAD and hit F5 to preview, F6 to render, export STL for printing.
//
// Assembly: star_front + star_back screw together around electronics.
// Handle bayonet-locks into star bottom. Twist off = power disconnect + USB-C exposed.

/* ── Tunable parameters ────────────────────────────────── */

// Star dimensions
star_outer_radius  = 35;    // tip-to-center (mm) — ~70mm tip-to-tip
star_inner_ratio   = 0.42;  // inner radius as fraction of outer (controls point depth)
star_points        = 5;
star_thickness     = 18;    // total thickness of star head (mm)
star_edge_fillet   = 2.0;   // rounding on edges

// Internal cavity for PCB + battery (centered in star)
cavity_width       = 28;    // CodeCell is ~25mm, plus clearance
cavity_depth       = 28;
cavity_height      = 12;    // room for PCB (~1.6mm) + LiPo (~5mm) + wiring

// Screw bosses — M2 brass heat-set inserts
screw_boss_d       = 5.5;   // outer diameter of boss
screw_hole_d       = 1.8;   // hole for M2 heat-set insert
screw_boss_count   = 4;
screw_boss_inset   = 8;     // distance from center to boss center

// PCB standoffs
standoff_h         = 2;     // height of standoffs inside cavity
standoff_d         = 4;     // standoff outer diameter
standoff_hole_d    = 1.6;   // M1.6 screw hole
pcb_mount_spacing  = 20;    // distance between mounting holes (estimate)

// Handle
handle_length      = 100;   // total handle length (mm)
handle_d_top       = 16;    // diameter at top (where it meets star)
handle_d_bottom    = 14;    // diameter at bottom (slight taper)
handle_fillet      = 1.5;

// Bayonet mount
bayonet_post_d     = 14;    // outer diameter of post on handle
bayonet_socket_d   = 14.4;  // inner diameter of socket in star (clearance fit)
bayonet_post_h     = 8;     // height of bayonet post
bayonet_pin_w      = 2.5;   // width of locking pin
bayonet_pin_h      = 2.0;   // height of pin
bayonet_pin_d      = 1.5;   // depth pin protrudes radially
bayonet_slot_arc   = 60;    // degrees of L-slot twist
bayonet_detent_r   = 0.4;   // small bump radius for detent click

// USB-C port cutout (bottom of star, covered by handle when assembled)
usbc_width         = 9.5;   // USB-C plug width + clearance
usbc_height        = 3.5;   // USB-C plug height + clearance
usbc_depth         = 8;     // how deep the cutout goes

// Contact pad recesses (for spring pins, future electronics integration)
contact_d          = 3;     // diameter of contact pad recess
contact_depth      = 1.5;   // depth of recess
contact_spacing    = 8;     // distance between +/- contacts

// Wall thickness
wall               = 2.0;   // minimum wall thickness everywhere

// Display options
show_cross_section = false;  // set true to see internal cross-section
show_assembly      = true;   // show assembled view
show_exploded      = false;  // show exploded view
show_handle        = true;
show_star_front    = true;
show_star_back     = true;
explode_distance   = 20;    // mm separation in exploded view

$fn = 80; // curve resolution

/* ── Modules ───────────────────────────────────────────── */

// 2D five-pointed star profile
module star_2d(outer_r, inner_r, points) {
    // Generate star polygon from alternating outer/inner points
    polygon([
        for (i = [0 : 2*points - 1])
            let(
                angle = 90 + i * 180 / points,
                r = (i % 2 == 0) ? outer_r : inner_r
            )
            [r * cos(angle), r * sin(angle)]
    ]);
}

// Rounded star — offset trick for filleted edges
module star_rounded_2d(outer_r, inner_r, points, fillet) {
    offset(r = fillet) offset(delta = -fillet)
        star_2d(outer_r, inner_r, points);
}

// 3D star body with rounded top/bottom edges
module star_body() {
    inner_r = star_outer_radius * star_inner_ratio;
    hull_r = star_edge_fillet;
    h = star_thickness - 2 * hull_r;

    minkowski() {
        linear_extrude(height = h)
            star_rounded_2d(
                star_outer_radius - hull_r,
                inner_r - hull_r,
                star_points,
                star_edge_fillet
            );
        sphere(r = hull_r);
    }
}

// Internal electronics cavity
module electronics_cavity() {
    translate([0, 0, (star_thickness - cavity_height) / 2 - star_edge_fillet])
        translate([-cavity_width/2, -cavity_depth/2, 0])
            cube([cavity_width, cavity_depth, cavity_height]);
}

// Screw boss positions (returned as list for reuse)
function screw_positions() = [
    [ screw_boss_inset,  screw_boss_inset],
    [-screw_boss_inset,  screw_boss_inset],
    [-screw_boss_inset, -screw_boss_inset],
    [ screw_boss_inset, -screw_boss_inset]
];

// Screw bosses (solid cylinders inside the cavity)
module screw_bosses(height) {
    positions = screw_positions();
    for (p = positions) {
        translate([p[0], p[1], 0])
            cylinder(d = screw_boss_d, h = height);
    }
}

// Screw holes through bosses
module screw_holes() {
    positions = screw_positions();
    for (p = positions) {
        translate([p[0], p[1], -1])
            cylinder(d = screw_hole_d, h = star_thickness + 2);
    }
}

// PCB standoffs (inside front half of cavity)
module pcb_standoffs() {
    spacing = pcb_mount_spacing / 2;
    positions = [
        [ spacing,  spacing],
        [-spacing,  spacing],
        [-spacing, -spacing],
        [ spacing, -spacing]
    ];
    for (p = positions) {
        translate([p[0], p[1], 0]) {
            difference() {
                cylinder(d = standoff_d, h = standoff_h);
                translate([0, 0, -0.1])
                    cylinder(d = standoff_hole_d, h = standoff_h + 0.2);
            }
        }
    }
}

// Bayonet socket (cut into bottom of star)
module bayonet_socket() {
    socket_h = bayonet_post_h + 0.5; // clearance

    // Main cylindrical socket
    translate([0, 0, -star_edge_fillet - 0.1])
        cylinder(d = bayonet_socket_d, h = socket_h + 0.1);

    // L-shaped slots (two, 180° apart)
    for (rot = [0, 180]) {
        rotate([0, 0, rot]) {
            // Vertical entry slot
            translate([0, 0, -star_edge_fillet - 0.1]) {
                translate([bayonet_socket_d/2 - bayonet_pin_d, -bayonet_pin_w/2, 0])
                    cube([bayonet_pin_d + 1, bayonet_pin_w, bayonet_pin_h + 0.5]);
            }
            // Horizontal twist slot
            translate([0, 0, -star_edge_fillet - 0.1]) {
                for (a = [0 : 2 : bayonet_slot_arc]) {
                    rotate([0, 0, a])
                        translate([bayonet_socket_d/2 - bayonet_pin_d, -bayonet_pin_w/2, 0])
                            cube([bayonet_pin_d + 1, bayonet_pin_w, bayonet_pin_h + 0.5]);
                }
            }
            // Detent pocket at end of twist
            rotate([0, 0, bayonet_slot_arc])
                translate([bayonet_socket_d/2, 0, bayonet_pin_h/2 - star_edge_fillet])
                    sphere(r = bayonet_detent_r + 0.15);
        }
    }
}

// USB-C port cutout
module usbc_cutout() {
    // Positioned at bottom of star, centered
    translate([-usbc_width/2, -usbc_depth, -star_edge_fillet - 0.1])
        cube([usbc_width, usbc_depth, usbc_height]);
}

// Contact pad recesses (in bayonet interface)
module contact_recesses() {
    for (x = [-contact_spacing/2, contact_spacing/2]) {
        translate([x, 0, -star_edge_fillet - 0.1])
            cylinder(d = contact_d, h = contact_depth);
    }
}

// ── Star front half (top) ──
module star_front() {
    split_z = star_thickness / 2 - star_edge_fillet;

    color("crimson", 0.85)
    difference() {
        // Top half of star body
        intersection() {
            star_body();
            translate([-star_outer_radius - 5, -star_outer_radius - 5, split_z])
                cube([star_outer_radius * 2 + 10, star_outer_radius * 2 + 10, star_thickness]);
        }
        // Cavity (top half)
        electronics_cavity();
        // Screw holes
        screw_holes();
    }
}

// ── Star back half (bottom) ──
module star_back() {
    split_z = star_thickness / 2 - star_edge_fillet;

    color("darkred", 0.85)
    difference() {
        union() {
            // Bottom half of star body
            intersection() {
                star_body();
                translate([-star_outer_radius - 5, -star_outer_radius - 5, -star_edge_fillet])
                    cube([star_outer_radius * 2 + 10, star_outer_radius * 2 + 10, split_z + star_edge_fillet]);
            }
            // Screw bosses inside cavity (growing up from bottom)
            translate([0, 0, split_z - cavity_height/2])
                screw_bosses(cavity_height / 2);
        }
        // Cavity (bottom half)
        electronics_cavity();
        // Screw holes
        screw_holes();
        // Bayonet socket
        bayonet_socket();
        // USB-C port
        usbc_cutout();
        // Contact pad recesses
        contact_recesses();
    }

    // PCB standoffs (on floor of cavity)
    color("darkred", 0.85)
    translate([0, 0, split_z - cavity_height/2])
        pcb_standoffs();
}

// ── Handle ──
module handle() {
    color("burlywood", 0.9)
    difference() {
        union() {
            // Tapered handle body
            translate([0, 0, -handle_length])
                cylinder(d1 = handle_d_bottom, d2 = handle_d_top, h = handle_length);

            // Bayonet post on top
            cylinder(d = bayonet_post_d, h = bayonet_post_h);

            // Locking pins (two, 180° apart)
            for (rot = [0, 180]) {
                rotate([0, 0, rot])
                    translate([bayonet_post_d/2 - bayonet_pin_d/2, -bayonet_pin_w/2, 0])
                        cube([bayonet_pin_d, bayonet_pin_w, bayonet_pin_h]);
            }

            // Detent bumps
            for (rot = [0, 180]) {
                rotate([0, 0, rot + bayonet_slot_arc])
                    translate([bayonet_post_d/2, 0, bayonet_pin_h/2])
                        sphere(r = bayonet_detent_r);
            }
        }

        // Contact pin holes (for spring pins)
        for (x = [-contact_spacing/2, contact_spacing/2]) {
            translate([x, 0, -0.1])
                cylinder(d = contact_d, h = bayonet_post_h + 0.2);
        }

        // Optional: hollow out handle bottom to save material
        translate([0, 0, -handle_length - 0.1])
            cylinder(d = handle_d_bottom - 2 * wall, h = handle_length - 10);
    }
}

/* ── Assembly ──────────────────────────────────────────── */

module assembly() {
    // Position everything relative to the star center
    // Star sits with its midplane at z=0 area, handle extends down

    if (show_exploded) {
        if (show_star_front)
            translate([0, 0, explode_distance]) star_front();
        if (show_star_back)
            star_back();
        if (show_handle)
            translate([0, 0, -explode_distance - star_edge_fillet])
                handle();
    } else {
        if (show_star_front) star_front();
        if (show_star_back)  star_back();
        if (show_handle)
            translate([0, 0, -star_edge_fillet])
                handle();
    }
}

/* ── Render ────────────────────────────────────────────── */

if (show_cross_section) {
    difference() {
        assembly();
        // Cut away front half for cross-section view
        translate([0, -star_outer_radius - 10, -handle_length - star_edge_fillet - 10])
            cube([star_outer_radius + 10, (star_outer_radius + 10) * 2, handle_length + star_thickness + 20]);
    }
} else {
    assembly();
}
