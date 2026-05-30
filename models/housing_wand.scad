// Wand Housing — Star head + handle, accepts core module via bayonet mount
// This is a dumb shell with no internal electronics — all smarts live in the module.
// Open in OpenSCAD: F5 to preview, F6 to render, export STL for printing.

include <mount.scad>

/* ── Star parameters ──────────────────────────────────── */

star_outer_radius  = 35;      // tip-to-center (mm)
star_inner_ratio   = 0.42;    // inner radius fraction (controls point depth)
star_points        = 5;
star_thickness     = 18;      // total thickness of star head (mm)
star_edge_fillet   = 2.0;     // rounding on edges

// Wall thickness
wall               = 2.5;     // slightly thicker than module — this is the outer toy shell

/* ── Handle parameters ────────────────────────────────── */

handle_length      = 100;     // total handle length (mm)
handle_d_top       = 18;      // diameter at top (where it meets star)
handle_d_bottom    = 14;      // diameter at bottom (slight taper)
handle_fillet      = 1.5;

// Handle-to-star attachment — fixed (glued/press-fit) for simplicity
// The module is the removable part now, not the handle.
handle_post_d      = 14;      // post that inserts into star
handle_post_h      = 8;       // insertion depth

/* ── Module socket position ───────────────────────────── */

// Module inserts from the back of the star (opposite the handle side)
// Socket is centered in the star, opening on the back face.
socket_z           = star_thickness - star_edge_fillet; // top surface of star

/* ── Display options ──────────────────────────────────── */

show_cross_section = false;
show_assembly      = true;
show_exploded      = false;
show_star          = true;
show_handle        = true;
show_module_ghost  = true;   // translucent module for fit-check
explode_distance   = 20;

$fn = 80;

/* ── Star geometry ────────────────────────────────────── */

// 2D five-pointed star profile
module star_2d(outer_r, inner_r, points) {
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

// Internal hollow — just needs to be big enough that the star is a shell,
// no electronics cavity needed (that's in the module now)
module star_hollow() {
    inner_r = star_outer_radius * star_inner_ratio;
    hull_r = star_edge_fillet;
    h = star_thickness - 2 * hull_r;
    inset = wall;

    translate([0, 0, wall])  // keep floor
    minkowski() {
        linear_extrude(height = h - wall)
            star_rounded_2d(
                star_outer_radius - hull_r - inset,
                inner_r - hull_r - inset,
                star_points,
                star_edge_fillet
            );
        sphere(r = hull_r * 0.5);
    }
}

/* ── Handle ───────────────────────────────────────────── */

module handle() {
    color("burlywood", 0.9)
    difference() {
        union() {
            // Tapered handle body
            translate([0, 0, -handle_length])
                cylinder(d1 = handle_d_bottom, d2 = handle_d_top, h = handle_length);

            // Post for star attachment
            cylinder(d = handle_post_d, h = handle_post_h);
        }

        // Hollow out handle to save material
        translate([0, 0, -handle_length - 0.1])
            cylinder(d = handle_d_bottom - 2 * wall, h = handle_length - 8);
    }
}

// Handle socket — subtracted from star bottom
module handle_socket() {
    translate([0, 0, -star_edge_fillet - 0.1])
        cylinder(d = handle_post_d + 0.4, h = handle_post_h + 0.5);
}

/* ── Module socket ────────────────────────────────────── */

// The bayonet socket is cut from the top of the star.
// Module inserts from above (the back face when held as a wand).
module module_socket() {
    translate([0, 0, socket_z - mount_socket_depth]) {
        mount_socket();
    }
}

// USB-C channel — runs from socket to the outer edge of the star
module usbc_access_channel() {
    translate([0, 0, socket_z - mount_socket_depth]) {
        mount_usbc_channel(wall);
    }
}

/* ── Ghost module (for fit visualization) ─────────────── */

module ghost_module() {
    // Import the module shape for visual fit-check
    // Using a simple cylinder approximation here
    module_d = 30;
    module_h = 18;
    module_fillet = 2.0;

    color("cyan", 0.25)
    translate([0, 0, socket_z - mount_socket_depth])
    hull() {
        translate([0, 0, module_fillet])
            rotate_extrude()
                translate([module_d/2 - module_fillet, 0, 0])
                    circle(r = module_fillet);
        translate([0, 0, module_h - module_fillet])
            rotate_extrude()
                translate([module_d/2 - module_fillet, 0, 0])
                    circle(r = module_fillet);
    }
}

/* ── Star assembly ────────────────────────────────────── */

module star() {
    color("crimson", 0.85)
    difference() {
        star_body();
        star_hollow();
        module_socket();
        usbc_access_channel();
        handle_socket();
    }
}

/* ── Full assembly ────────────────────────────────────── */

module assembly() {
    if (show_exploded) {
        if (show_star) star();
        if (show_handle)
            translate([0, 0, -explode_distance - star_edge_fillet])
                handle();
        if (show_module_ghost)
            translate([0, 0, explode_distance])
                ghost_module();
    } else {
        if (show_star) star();
        if (show_handle)
            translate([0, 0, -star_edge_fillet])
                handle();
        if (show_module_ghost)
            ghost_module();
    }
}

/* ── Render ────────────────────────────────────────────── */

if (show_cross_section) {
    difference() {
        assembly();
        translate([0, -star_outer_radius - 10, -handle_length - star_edge_fillet - 10])
            cube([star_outer_radius + 10, (star_outer_radius + 10) * 2,
                  handle_length + star_thickness + 20]);
    }
} else {
    assembly();
}
