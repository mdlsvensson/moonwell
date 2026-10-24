namespace Moonwell
{
    /// <summary>
    /// The first hit along a segment from one point to another, against a circle (a unit's collision), a wall, bounds or a
    /// polygon. Each returns whether it hit, with the hit in an out value (default on a miss), so nothing is made when nothing
    /// is hit. Starting inside the shape is a hit at fraction 0, with the normal against the segment's direction.
    /// </summary>
    public static class Raycast
    {
        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a circle.</summary>
        [LocalSafe]
        public static bool Circle(Vector2 from, Vector2 to, Vector2 center, float radius, out RayHit hit)
        {
            var dx = to.X - from.X;
            var dy = to.Y - from.Y;
            var fx = from.X - center.X;
            var fy = from.Y - center.Y;
            var c = fx * fx + fy * fy - radius * radius;
            if (c <= 0f)
            {
                hit = Inside(from, dx, dy);
                return true;
            }

            var a = dx * dx + dy * dy;
            if (a == 0f)
            {
                hit = default;
                return false;
            }

            var b = 2f * (fx * dx + fy * dy);
            var discriminant = b * b - 4f * a * c;
            if (discriminant < 0f)
            {
                hit = default;
                return false;
            }

            var t = (-b - Trig.Sqrt(discriminant)) / (2f * a);
            if (t < 0f || t > 1f)
            {
                hit = default;
                return false;
            }

            var point = new Vector2(from.X + dx * t, from.Y + dy * t);
            hit = new RayHit(point, new Vector2((point.X - center.X) / radius, (point.Y - center.Y) / radius), t);
            return true;
        }

        /// <summary>The hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a wall from a to b. Parallel and collinear segments do not hit.</summary>
        [LocalSafe]
        public static bool Segment(Vector2 from, Vector2 to, Vector2 a, Vector2 b, out RayHit hit)
        {
            var rx = to.X - from.X;
            var ry = to.Y - from.Y;
            var sx = b.X - a.X;
            var sy = b.Y - a.Y;
            var denominator = rx * sy - ry * sx;
            if (denominator == 0f)
            {
                hit = default;
                return false;
            }

            var qx = a.X - from.X;
            var qy = a.Y - from.Y;
            var t = (qx * sy - qy * sx) / denominator;
            var u = (qx * ry - qy * rx) / denominator;
            if (t < 0f || t > 1f || u < 0f || u > 1f)
            {
                hit = default;
                return false;
            }

            var normal = new Vector2(-sy, sx).Normalized();
            if (normal.X * rx + normal.Y * ry > 0f)
            {
                normal = -normal;
            }

            hit = new RayHit(new Vector2(from.X + rx * t, from.Y + ry * t), normal, t);
            return true;
        }

        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on bounds.</summary>
        [LocalSafe]
        public static bool Bounds(Vector2 from, Vector2 to, Bounds bounds, out RayHit hit)
        {
            if (bounds.Contains(from))
            {
                hit = Inside(from, to.X - from.X, to.Y - from.Y);
                return true;
            }

            var min = bounds.Min;
            var max = bounds.Max;
            var found = false;
            hit = default;
            Nearer(from, to, min, new Vector2(max.X, min.Y), ref found, ref hit);
            Nearer(from, to, new Vector2(max.X, min.Y), max, ref found, ref hit);
            Nearer(from, to, max, new Vector2(min.X, max.Y), ref found, ref hit);
            Nearer(from, to, new Vector2(min.X, max.Y), min, ref found, ref hit);
            return found;
        }

        /// <summary>The first hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a polygon's edges.</summary>
        [LocalSafe]
        public static bool Polygon(Vector2 from, Vector2 to, Polygon polygon, out RayHit hit)
        {
            if (!polygon.Bounds.Intersects(new Bounds(from, to)))
            {
                hit = default;
                return false;
            }

            if (polygon.Contains(from))
            {
                hit = Inside(from, to.X - from.X, to.Y - from.Y);
                return true;
            }

            var points = polygon.Points;
            var count = points.Count;
            var found = false;
            hit = default;
            for (var i = 0; i < count; i++)
            {
                Nearer(from, to, points[i], points[(i + 1) % count], ref found, ref hit);
            }

            return found;
        }

        /// <summary>The hit at the start for a segment that starts inside: the normal points against its direction.</summary>
        private static RayHit Inside(Vector2 from, float dx, float dy)
        {
            return new RayHit(from, new Vector2(-dx, -dy).Normalized(), 0f);
        }

        /// <summary>Keeps the hit on the edge from a to b if it is the first so far.</summary>
        private static void Nearer(Vector2 from, Vector2 to, Vector2 a, Vector2 b, ref bool found, ref RayHit nearest)
        {
            if (Segment(from, to, a, b, out var hit) && (!found || hit.Fraction < nearest.Fraction))
            {
                nearest = hit;
                found = true;
            }
        }
    }
}
