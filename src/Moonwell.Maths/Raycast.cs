namespace Moonwell
{
    /// <summary>
    /// The first hit along a segment from one point to another, against a circle (a unit's collision), a wall, bounds or a
    /// polygon. Each returns whether it hit, with the hit in an out value (an empty hit on a miss), so nothing is made when
    /// nothing is hit. A map can move its object to a hit's point and bounce or slide it there: a segment that starts on a
    /// shape's surface (within 0.02 of it) hits it only if it goes in, deeper than 0.01 and deeper than it started, and then
    /// at once, on the surface, with the surface's normal; one that moves out or along it does not hit it. A segment that
    /// starts deeper inside is a hit at fraction 0, with the normal against its direction. A wall has no inside: a segment
    /// that starts on it (within 0.01) does not hit it, whichever way it goes, and one that comes toward it and ends on it
    /// hits it at its end.
    /// </summary>
    public static class Raycast
    {
        /// <summary>
        /// How far into a shape past its surface a segment that starts on the surface must go to hit it, and how thick a wall
        /// is: well above a float's rounding of a hit's point anywhere on the largest map (a float's step at 30,000 is
        /// 0.002), and well below anything the game shows.
        /// </summary>
        private const float Skin = 0.01f;

        /// <summary>
        /// How close to a shape's surface a start counts as on it: twice the skin, so a start that the rounding of a map's
        /// steps has carried a little past the skin is still on the surface, and the next hit puts it back on it.
        /// </summary>
        private const float Surface = 0.02f;

        /// <summary>The empty hit every miss gives, made on first use and shared (a RayHit never changes).</summary>
        private static RayHit[]? none;

        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a circle.</summary>
        [LocalSafe]
        public static bool Circle(Vector2 from, Vector2 to, Vector2 center, float radius, out RayHit hit)
        {
            var dx = to.X - from.X;
            var dy = to.Y - from.Y;
            var fx = from.X - center.X;
            var fy = from.Y - center.Y;
            var squared = fx * fx + fy * fy;
            var deep = radius - Surface;
            if (deep > 0f && squared < deep * deep)
            {
                hit = Inside(from, dx, dy);
                return true;
            }

            var a = dx * dx + dy * dy;
            var b = 2f * (fx * dx + fy * dy);
            var c = squared - radius * radius;
            var outer = radius + Surface;
            if (squared <= outer * outer)
            {
                // On the surface: a hit only if it goes in, nearer the center than the skin somewhere along it.
                var inner = radius - Skin;
                if (!(b < 0f) || !(inner > 0f))
                {
                    hit = Miss();
                    return false;
                }

                var k = -b / (2f * a);
                if (k > 1f)
                {
                    k = 1f;
                }

                var nearestX = fx + dx * k;
                var nearestY = fy + dy * k;
                if (!(nearestX * nearestX + nearestY * nearestY < inner * inner))
                {
                    hit = Miss();
                    return false;
                }

                // Where it crosses the circle, or at its start if that is inside: then on the circle beside it, so a map that
                // moves its object to the hit's point keeps it on the surface, however a float rounds its steps.
                var at = c > 0f ? (-b - Trig.Sqrt(b * b - 4f * a * c)) / (2f * a) : 0f;
                var px = fx + dx * at;
                var py = fy + dy * at;
                var length = Trig.Sqrt(px * px + py * py);
                var nx = px / length;
                var ny = py / length;
                var on = c > 0f ? new Vector2(from.X + dx * at, from.Y + dy * at) : new Vector2(center.X + nx * radius, center.Y + ny * radius);
                hit = new RayHit(on, new Vector2(nx, ny), at);
                return true;
            }

            if (a == 0f)
            {
                hit = Miss();
                return false;
            }

            var discriminant = b * b - 4f * a * c;
            if (discriminant < 0f)
            {
                hit = Miss();
                return false;
            }

            var t = (-b - Trig.Sqrt(discriminant)) / (2f * a);
            if (t < 0f || t > 1f)
            {
                hit = Miss();
                return false;
            }

            var point = new Vector2(from.X + dx * t, from.Y + dy * t);
            hit = new RayHit(point, new Vector2((point.X - center.X) / radius, (point.Y - center.Y) / radius), t);
            return true;
        }

        /// <summary>
        /// The hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a wall from a to b. Parallel and
        /// collinear segments do not hit. One that starts on the wall (within 0.01) does not hit it, whichever way it goes;
        /// one that comes toward it and ends on it hits it at its end.
        /// </summary>
        [LocalSafe]
        public static bool Segment(Vector2 from, Vector2 to, Vector2 a, Vector2 b, out RayHit hit)
        {
            if (OnWall(from.X, from.Y, a.X, a.Y, b.X, b.Y))
            {
                hit = Miss();
                return false;
            }

            if (!Meets(from, to, a.X, a.Y, b.X, b.Y, false, out var t))
            {
                if (!OnWall(to.X, to.Y, a.X, a.Y, b.X, b.Y) || !Toward(from, to, a.X, a.Y, b.X, b.Y))
                {
                    hit = Miss();
                    return false;
                }

                t = 1f;
            }

            hit = HitOn(from, to, a.X, a.Y, b.X, b.Y, t);
            return true;
        }

        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on bounds.</summary>
        [LocalSafe]
        public static bool Bounds(Vector2 from, Vector2 to, Bounds bounds, out RayHit hit)
        {
            if (Deep(bounds, from.X, from.Y, Surface))
            {
                hit = Inside(from, to.X - from.X, to.Y - from.Y);
                return true;
            }

            float minX = bounds.Min.X, minY = bounds.Min.Y, maxX = bounds.Max.X, maxY = bounds.Max.Y;
            var surface = from.X >= minX - Surface && from.X <= maxX + Surface && from.Y >= minY - Surface && from.Y <= maxY + Surface;
            var best = 2f;
            var edge = -1;
            Nearer(from, to, minX, minY, maxX, minY, 0, surface, null, bounds, ref best, ref edge);
            Nearer(from, to, maxX, minY, maxX, maxY, 1, surface, null, bounds, ref best, ref edge);
            Nearer(from, to, maxX, maxY, minX, maxY, 2, surface, null, bounds, ref best, ref edge);
            Nearer(from, to, minX, maxY, minX, minY, 3, surface, null, bounds, ref best, ref edge);
            if (edge < 0)
            {
                hit = Miss();
                return false;
            }

            if (edge == 0)
            {
                hit = HitOn(from, to, minX, minY, maxX, minY, best);
            }
            else if (edge == 1)
            {
                hit = HitOn(from, to, maxX, minY, maxX, maxY, best);
            }
            else if (edge == 2)
            {
                hit = HitOn(from, to, maxX, maxY, minX, maxY, best);
            }
            else
            {
                hit = HitOn(from, to, minX, maxY, minX, minY, best);
            }

            return true;
        }

        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a polygon's edges.</summary>
        [LocalSafe]
        public static bool Polygon(Vector2 from, Vector2 to, Polygon polygon, out RayHit hit)
        {
            var box = polygon.Bounds;
            float minX = box.Min.X - Surface, minY = box.Min.Y - Surface, maxX = box.Max.X + Surface, maxY = box.Max.Y + Surface;
            if ((from.X < minX && to.X < minX) || (from.X > maxX && to.X > maxX)
                || (from.Y < minY && to.Y < minY) || (from.Y > maxY && to.Y > maxY))
            {
                hit = Miss();
                return false;
            }

            var surface = false;
            if (from.X >= minX && from.X <= maxX && from.Y >= minY && from.Y <= maxY)
            {
                surface = !polygon.Clear(from.X, from.Y, Surface);
                if (!surface && polygon.Contains(from.X, from.Y))
                {
                    hit = Inside(from, to.X - from.X, to.Y - from.Y);
                    return true;
                }
            }

            var points = polygon.Points;
            var count = points.Count;
            var best = 2f;
            var edge = -1;
            for (var i = 0; i < count; i++)
            {
                var a = points[i];
                var b = points[(i + 1) % count];
                Nearer(from, to, a.X, a.Y, b.X, b.Y, i, surface, polygon, box, ref best, ref edge);
            }

            if (edge < 0)
            {
                hit = Miss();
                return false;
            }

            var first = points[edge];
            var second = points[(edge + 1) % count];
            hit = HitOn(from, to, first.X, first.Y, second.X, second.Y, best);
            return true;
        }

        /// <summary>The empty hit of a miss: one shared value, so a miss makes nothing.</summary>
        private static RayHit Miss()
        {
            if (none == null)
            {
                none = new[] { new RayHit(Vector2.Zero, Vector2.Zero, 0f) };
            }

            return none[0];
        }

        /// <summary>The hit at the start for a segment that starts deeper inside than the surface: the normal points against its direction.</summary>
        private static RayHit Inside(Vector2 from, float dx, float dy)
        {
            return new RayHit(from, new Vector2(-dx, -dy).Normalized(), 0f);
        }

        /// <summary>Whether a point is inside bounds by more than a distance.</summary>
        private static bool Deep(Bounds bounds, float x, float y, float distance)
        {
            return x > bounds.Min.X + distance && x < bounds.Max.X - distance && y > bounds.Min.Y + distance && y < bounds.Max.Y - distance;
        }

        /// <summary>Whether a point is on the wall from (ax, ay) to (bx, by): within the skin of it.</summary>
        private static bool OnWall(float x, float y, float ax, float ay, float bx, float by)
        {
            return Moonwell.Polygon.Near(x, y, ax, ay, bx, by, Skin);
        }

        /// <summary>
        /// Whether the segment from <paramref name="from"/> to <paramref name="to"/> meets the wall from (ax, ay) to (bx, by),
        /// and at what fraction; worked out in floats, so nothing is made. When the segment starts on the surface of the
        /// shape the wall belongs to, near the wall's line, it meets the wall only if it goes on past the skin and deeper
        /// than it started: where it crosses the line, or at its start if it does not cross it.
        /// </summary>
        private static bool Meets(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, bool surface, out float t)
        {
            t = 0f;
            var sx = bx - ax;
            var sy = by - ay;
            var lengthSquared = sx * sx + sy * sy;
            if (lengthSquared == 0f)
            {
                return false;
            }

            // How far the start and the end are from the wall's line, on one side or the other, times the wall's length.
            var start = (from.X - ax) * sy - (from.Y - ay) * sx;
            var end = (to.X - ax) * sy - (to.Y - ay) * sx;
            var skin = Skin * Skin * lengthSquared;
            if (surface && start * start <= Surface * Surface * lengthSquared)
            {
                if (!(end * end > skin))
                {
                    return false;
                }

                if ((start > 0f && end < 0f) || (start < 0f && end > 0f))
                {
                    t = start / (start - end);
                }
                else if (!(end * end > start * start))
                {
                    return false;
                }
            }
            else
            {
                if (start == end || (start > 0f && end > 0f) || (start < 0f && end < 0f))
                {
                    return false;
                }

                t = start / (start - end);
                if (!(t >= 0f && t <= 1f))
                {
                    return false;
                }
            }

            var along = (from.X + (to.X - from.X) * t - ax) * sx + (from.Y + (to.Y - from.Y) * t - ay) * sy;
            return along >= 0f && along <= lengthSquared;
        }

        /// <summary>Whether the segment's end is nearer the line of the wall from (ax, ay) to (bx, by) than its start: it comes toward the wall.</summary>
        private static bool Toward(Vector2 from, Vector2 to, float ax, float ay, float bx, float by)
        {
            var sx = bx - ax;
            var sy = by - ay;
            var start = (from.X - ax) * sy - (from.Y - ay) * sx;
            var end = (to.X - ax) * sy - (to.Y - ay) * sx;
            return end * end < start * start;
        }

        /// <summary>
        /// The point twice the skin past the line of the wall from (ax, ay) to (bx, by), on the side the segment goes to; its
        /// start if that is deeper, its end if that is nearer: past the wall, which a segment that goes into the shape there
        /// has reached.
        /// </summary>
        private static void Probe(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, out float x, out float y)
        {
            var sx = bx - ax;
            var sy = by - ay;
            var start = (from.X - ax) * sy - (from.Y - ay) * sx;
            var end = (to.X - ax) * sy - (to.Y - ay) * sx;
            var depth = 2f * Skin * Trig.Sqrt(sx * sx + sy * sy);
            var t = (start - (end > 0f ? depth : -depth)) / (start - end);
            if (!(t < 1f))
            {
                t = 1f;
            }

            if (t < 0f)
            {
                t = 0f;
            }

            x = from.X + (to.X - from.X) * t;
            y = from.Y + (to.Y - from.Y) * t;
        }

        /// <summary>
        /// The hit at fraction t on the wall from (ax, ay) to (bx, by), with the wall's normal facing back toward the start.
        /// A point behind the wall's line (a segment that starts just inside the surface and goes in) is put on the line, so
        /// a map that moves its object to the hit's point keeps it on the surface, however a float rounds its steps.
        /// </summary>
        private static RayHit HitOn(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, float t)
        {
            var rx = to.X - from.X;
            var ry = to.Y - from.Y;
            var normal = new Vector2(ay - by, bx - ax).Normalized();
            if (normal.X * rx + normal.Y * ry > 0f)
            {
                normal = -normal;
            }

            var x = from.X + rx * t;
            var y = from.Y + ry * t;
            var behind = normal.X * (x - ax) + normal.Y * (y - ay);
            if (behind < 0f)
            {
                x -= normal.X * behind;
                y -= normal.Y * behind;
            }

            return new RayHit(new Vector2(x, y), normal, t);
        }

        /// <summary>
        /// Keeps the wall's fraction and number if the segment meets it nearer than the best so far. For a segment that starts
        /// on the shape's surface, only if it goes into the shape there: the point past the wall is inside the shape (the
        /// polygon, or the bounds when there is no polygon) by more than half the skin, so not along another of its walls.
        /// </summary>
        private static void Nearer(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, int number, bool surface, Polygon? polygon, Bounds box, ref float best, ref int edge)
        {
            if (!Meets(from, to, ax, ay, bx, by, surface, out var t) || !(t < best))
            {
                return;
            }

            if (surface)
            {
                Probe(from, to, ax, ay, bx, by, out var x, out var y);
                var inside = polygon != null ? polygon.Contains(x, y) && polygon.Clear(x, y, Skin / 2f) : Deep(box, x, y, Skin / 2f);
                if (!inside)
                {
                    return;
                }
            }

            best = t;
            edge = number;
        }
    }
}
