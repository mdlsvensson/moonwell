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

            var min = points[0];
            var max = points[0];
            var twiceArea = 0f;
            var cx = 0f;
            var cy = 0f;
            for (var i = 0; i < points.Length; i++)
            {
                var a = points[i];
                var b = points[(i + 1) % points.Length];
                min = new Vector2(MathF.Min(min.X, a.X), MathF.Min(min.Y, a.Y));
                max = new Vector2(MathF.Max(max.X, a.X), MathF.Max(max.Y, a.Y));
                var cross = a.Cross(b);
                twiceArea += cross;
                cx += (a.X + b.X) * cross;
                cy += (a.Y + b.Y) * cross;
            }

            Bounds = new Bounds(min, max);
            Area = MathF.Abs(twiceArea) / 2f;
            if (twiceArea == 0f)
            {
                // No area (the points lie on a line): the plain average.
                var sx = 0f;
                var sy = 0f;
                for (var i = 0; i < points.Length; i++)
                {
                    sx += points[i].X;
                    sy += points[i].Y;
                }

                Center = new Vector2(sx / points.Length, sy / points.Length);
            }
            else
            {
                Center = new Vector2(cx / (3f * twiceArea), cy / (3f * twiceArea));
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
            if (!Bounds.Contains(point))
            {
                return false;
            }

            var inside = false;
            var count = points.Length;
            for (int i = 0, j = count - 1; i < count; j = i++)
            {
                var a = points[i];
                var b = points[j];
                if (IsOnSegment(point, a, b))
                {
                    return true;
                }

                if ((a.Y > point.Y) != (b.Y > point.Y)
                    && point.X < (b.X - a.X) * (point.Y - a.Y) / (b.Y - a.Y) + a.X)
                {
                    inside = !inside;
                }
            }

            return inside;
        }

        /// <summary>Whether a point lies on the segment from a to b, within OnEdge.</summary>
        private static bool IsOnSegment(Vector2 point, Vector2 a, Vector2 b)
        {
            var abx = b.X - a.X;
            var aby = b.Y - a.Y;
            var lengthSquared = abx * abx + aby * aby;
            if (lengthSquared == 0f)
            {
                return point.DistanceSquaredTo(a) <= OnEdge * OnEdge;
            }

            var t = Maths.Clamp01(((point.X - a.X) * abx + (point.Y - a.Y) * aby) / lengthSquared);
            var nearestX = a.X + abx * t;
            var nearestY = a.Y + aby * t;
            return Maths.DistanceSquared(point.X, point.Y, nearestX, nearestY) <= OnEdge * OnEdge;
        }
    }
}
