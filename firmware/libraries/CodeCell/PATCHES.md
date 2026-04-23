# Local patches

This copy of the CodeCell library carries a small local patch on top of
upstream (https://github.com/microbotsio/CodeCell):

## `Motion_RotationNoMagVectorRead`

Adds a public accessor for the BNO085 game rotation vector quaternion
(w, x, y, z). The upstream library stores this in `_motion_data[4..7]` but
only exposes it through `Motion_RotationNoMagRead`, which decomposes it to
Euler angles and loses frame information needed by the wand project's games.

- `src/CodeCell.h`: one-line public declaration
- `src/CodeCell.cpp`: implementation mirroring `Motion_RotationNoMagRead`
  but returning the raw quaternion

Both sites are commented with `Patched:` so they can be diffed against
upstream.

If upstream accepts an equivalent PR this patch can be removed.
