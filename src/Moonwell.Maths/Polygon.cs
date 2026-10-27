using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>
    /// A shape on the map with straight edges, from three points or more in either order. It never changes once made: it keeps
    /// its own copy of the points, and works out its bounds, area and center once. Make one and keep it, as a wall or an
    /// area. A self-crossing polygon counts a point inside by the even-odd rule, and its Area and Center mean little.
    /// </summary>
    public sealed class Polygon
    {
        /// <summary>How close to an edge a point counts as on it.</summary>
        private const float OnEdge = 0.001f;

        private readonly Vector2[] points;

        /// <summary>Makes a polygon from its corners, in either order. Fewer than three throws an ArgumentException.</summary>
        [LocalSafe]
        public Polygon(params Vector2[] points)
        {
            if (points == null || points.Length < 3)
            {
                throw new ArgumentException("A polygon needs at least three points.");
            }

            this.points = new Vector2[points.Length];
            for (var i = 0; i < points.Length; i++)
            {
                this.points[i] = points[i];
            }

            // The sums are worked out from the first point, not from (0, 0): far out, the cross products of whole
            // coordinates are in the hundreds of millions, and a float's rounding of them is more than a small polygon's area.
            var originX = points[0].X;
            var originY = points[0].Y;
            var minX = originX;
            var minY = originY;
            var maxX = originX;
            var maxY = originY;
            var twiceArea = 0f;
            var cx = 0f;
            var cy = 0f;
            for (var i = 0; i < points.Length; i++)
            {
                var a = points[i];
                var b = points[(i + 1) % points.Length];
                minX = MathF.Min(minX, a.X);
                minY = MathF.Min(minY, a.Y);
                maxX = MathF.Max(maxX, a.X);
                maxY = MathF.Max(maxY, a.Y);
                var ax = a.X - originX;
                var ay = a.Y - originY;
                var bx = b.X - originX;
                var by = b.Y - originY;
                var cross = ax * by - ay * bx;
                twiceArea += cross;
                cx += (ax + bx) * cross;
                cy += (ay + by) * cross;
            }

            Bounds = new Bounds(new Vector2(minX, minY), new Vector2(maxX, maxY));
            Area = MathF.Abs(twiceArea) / 2f;
            if (twiceArea == 0f)
            {
                // No area (the points lie on a line): the plain average.
                var sx = 0f;
                var sy = 0f;
                for (var i = 0; i < points.Length; i++)
                {
                    sx += points[i].X - originX;
                    sy += points[i].Y - originY;
                }

                Center = new Vector2(originX + sx / points.Length, originY + sy / points.Length);
            }
            else
            {
                Center = new Vector2(originX + cx / (3f * twiceArea), originY + cy / (3f * twiceArea));
            }
        }

        /// <summary>The corners, in the order given.</summary>
        [LocalSafe]
        public IReadOnlyList<Vector2> Points => points;

        /// <summary>The smallest bounds holding every corner.</summary>
        [LocalSafe]
        public Bounds Bounds { get; }

        /// <summary>The area, positive in either order of the corners.</summary>
        [LocalSafe]
        public float Area { get; }

        /// <summary>The center of the area (its centroid).</summary>
        [LocalSafe]
        public Vector2 Center { get; }

        /// <summary>Whether a point is inside; a point on an edge counts as inside.</summary>
        [LocalSafe]
        public bool Contains(Vector2 point)
        {
            return Contains(point.X, point.Y);
        }

        /// <summary>Whether a point is inside, given as floats so that no vector is made; a point on an edge counts as inside.</summary>
        [LocalSafe]
        public bool Contains(float x, float y)
        {
            if (x < Bounds.Min.X || x > Bounds.Max.X || y < Bounds.Min.Y || y > Bounds.Max.Y)
            {
                return false;
            }

            var inside = false;
            var count = points.Length;
            for (int i = 0, j = count - 1; i < count; j = i++)
            {
                var ax = points[i].X;
                var ay = points[i].Y;
                var bx = points[j].X;
                var by = points[j].Y;
                if (Near(x, y, ax, ay, bx, by, OnEdge))
                {
                    return true;
                }

                if ((ay > y) != (by > y)
                    && x < (bx - ax) * (y - ay) / (by - ay) + ax)
                {
                    inside = !inside;
                }
            }

            return inside;
        }

        /// <summary>Whether a point is farther than a distance from every edge, given as floats so that no vector is made; the raycasts' test of a point deeper inside than the surface.</summary>
        internal bool Clear(float x, float y, float distance)
        {
            var count = points.Length;
            for (int i = 0, j = count - 1; i < count; j = i++)
            {
                if (Near(x, y, points[i].X, points[i].Y, points[j].X, points[j].Y, distance))
                {
                    return false;
                }
            }

            return true;
        }

        /// <summary>
        /// Whether the point (x, y) lies within a distance of the segment from (ax, ay) to (bx, by). Worked out from (ax, ay),
        /// so a point far from (0, 0) keeps its small distance (a float's step there is up to 0.004).
        /// </summary>
        internal static bool Near(float x, float y, float ax, float ay, float bx, float by, float distance)
        {
            var abx = bx - ax;
            var aby = by - ay;
            var px = x - ax;
            var py = y - ay;
            var lengthSquared = abx * abx + aby * aby;
            if (lengthSquared == 0f)
            {
                return px * px + py * py <= distance * distance;
            }

            var t = Maths.Clamp01((px * abx + py * aby) / lengthSquared);
            var ex = px - abx * t;
            var ey = py - aby * t;
            return ex * ex + ey * ey <= distance * distance;
        }
    }
}
