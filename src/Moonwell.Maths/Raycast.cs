namespace Moonwell
{
    /// <summary>
    /// The first hit along a segment from one point to another, against a circle (a unit's collision), a wall, bounds or a
    /// polygon. Each returns whether it hit, with the hit in an out value (an empty hit on a miss), so nothing is made when
    /// nothing is hit. Starting inside the shape is a hit at fraction 0, with the normal against the segment's direction.
    /// </summary>
    public static class Raycast
    {
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
            var c = fx * fx + fy * fy - radius * radius;
            if (c <= 0f)
            {
                hit = Inside(from, dx, dy);
                return true;
            }

            var a = dx * dx + dy * dy;
            if (a == 0f)
            {
                hit = Miss();
                return false;
            }

            var b = 2f * (fx * dx + fy * dy);
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

        /// <summary>The hit of the segment from <paramref name="from"/> to <paramref name="to"/> on a wall from a to b. Parallel and collinear segments do not hit.</summary>
        [LocalSafe]
        public static bool Segment(Vector2 from, Vector2 to, Vector2 a, Vector2 b, out RayHit hit)
        {
            if (!Crosses(from, to, a.X, a.Y, b.X, b.Y, out var t))
            {
                hit = Miss();
                return false;
            }

            hit = HitOn(from, to, a.X, a.Y, b.X, b.Y, t);
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

            float minX = bounds.Min.X, minY = bounds.Min.Y, maxX = bounds.Max.X, maxY = bounds.Max.Y;
            var best = 2f;
            var edge = -1;
            Nearer(from, to, minX, minY, maxX, minY, 0, ref best, ref edge);
            Nearer(from, to, maxX, minY, maxX, maxY, 1, ref best, ref edge);
            Nearer(from, to, maxX, maxY, minX, maxY, 2, ref best, ref edge);
            Nearer(from, to, minX, maxY, minX, minY, 3, ref best, ref edge);
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
            if ((from.X < box.Min.X && to.X < box.Min.X) || (from.X > box.Max.X && to.X > box.Max.X)
                || (from.Y < box.Min.Y && to.Y < box.Min.Y) || (from.Y > box.Max.Y && to.Y > box.Max.Y))
            {
                hit = Miss();
                return false;
            }

            if (polygon.Contains(from))
            {
                hit = Inside(from, to.X - from.X, to.Y - from.Y);
                return true;
            }

            var points = polygon.Points;
            var count = points.Count;
            var best = 2f;
            var edge = -1;
            for (var i = 0; i < count; i++)
            {
                var a = points[i];
                var b = points[(i + 1) % count];
                Nearer(from, to, a.X, a.Y, b.X, b.Y, i, ref best, ref edge);
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

        /// <summary>The hit at the start for a segment that starts inside: the normal points against its direction.</summary>
        private static RayHit Inside(Vector2 from, float dx, float dy)
        {
            return new RayHit(from, new Vector2(-dx, -dy).Normalized(), 0f);
        }

        /// <summary>Whether the segment from <paramref name="from"/> to <paramref name="to"/> crosses the wall from (ax, ay) to (bx, by), and at what fraction; worked out in floats, so nothing is made.</summary>
        private static bool Crosses(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, out float t)
        {
            var rx = to.X - from.X;
            var ry = to.Y - from.Y;
            var sx = bx - ax;
            var sy = by - ay;
            var denominator = rx * sy - ry * sx;
            if (denominator == 0f)
            {
                t = 0f;
                return false;
            }

            var qx = ax - from.X;
            var qy = ay - from.Y;
            t = (qx * sy - qy * sx) / denominator;
            var u = (qx * ry - qy * rx) / denominator;
            return t >= 0f && t <= 1f && u >= 0f && u <= 1f;
        }

        /// <summary>The hit at fraction t on the wall from (ax, ay) to (bx, by), with the wall's normal facing back toward the start.</summary>
        private static RayHit HitOn(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, float t)
        {
            var rx = to.X - from.X;
            var ry = to.Y - from.Y;
            var normal = new Vector2(ay - by, bx - ax).Normalized();
            if (normal.X * rx + normal.Y * ry > 0f)
            {
                normal = -normal;
            }

            return new RayHit(new Vector2(from.X + rx * t, from.Y + ry * t), normal, t);
        }

        /// <summary>Keeps the wall's fraction and number if it is crossed nearer than the best so far.</summary>
        private static void Nearer(Vector2 from, Vector2 to, float ax, float ay, float bx, float by, int number, ref float best, ref int edge)
        {
            if (Crosses(from, to, ax, ay, bx, by, out var t) && t < best)
            {
                best = t;
                edge = number;
            }
        }
    }
}
