using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>
    /// A shape on the map that an area covers: a World Editor region, a rect, a polygon, a circle, or several of those
    /// together. It never changes once made. <c>Contains</c> tests the true shape, but the game sees an area in whole
    /// 32 by 32 cells: a rect is rounded out to whole cells, and a polygon or a circle is the cells whose centre is
    /// inside it, and always the cell its own centre is in. So a unit can be inside the shape by <c>Contains</c> and not
    /// yet in the area, or the other way round, within a cell of its edge; a circle of radius under 23 or a polygon
    /// thinner than a cell may be only the one cell its centre is in.
    /// </summary>
    public sealed class AreaShape
    {
        /// <summary>The ways a shape is made, which decide how the game is given it.</summary>
        internal enum ShapeKind
        {
            /// <summary>A rect: a World Editor region, or bounds.</summary>
            Rect,

            /// <summary>A polygon, as cells.</summary>
            Polygon,

            /// <summary>A circle, as cells.</summary>
            Circle,

            /// <summary>Several shapes together.</summary>
            Union,
        }

        /// <summary>How the shape was made.</summary>
        internal readonly ShapeKind Kind;

        /// <summary>The polygon of a polygon shape; null otherwise.</summary>
        internal readonly Polygon? Corners;

        /// <summary>The centre of a circle.</summary>
        internal readonly Vector2 Centre;

        /// <summary>The radius of a circle.</summary>
        internal readonly float Radius;

        /// <summary>The shapes of a union, in the order given; empty otherwise.</summary>
        internal readonly AreaShape[] Parts;

        /// <summary>The World Editor region's name of a region shape; null for the others.</summary>
        private readonly string? regionName;

        /// <summary>The bounds, once known: made with the shape, except for a region and for shapes holding one, which are worked out on first use.</summary>
        private Bounds bounds;

        /// <summary>Whether <c>bounds</c> is known.</summary>
        private bool resolved;

        private AreaShape(ShapeKind kind, Bounds bounds, bool resolved, string? regionName, Polygon? corners, Vector2 centre, float radius, AreaShape[] parts)
        {
            Kind = kind;
            this.bounds = bounds;
            this.resolved = resolved;
            this.regionName = regionName;
            Corners = corners;
            Centre = centre;
            Radius = radius;
            Parts = parts;
        }

        /// <summary>
        /// A World Editor region by the name it has there. The game gives the area the region's rect rounded out to
        /// whole 32 cells. The shape may be made before the package is installed (a map's constructors run while it is
        /// built); the region is looked up at the first read of its members, or when the area is added. An unknown name
        /// then throws ArgumentException, and a read before the package is installed throws InvalidOperationException.
        /// The build checks a name written as a literal against the map's regions.
        /// </summary>
        [LocalSafe]
        public static AreaShape Region(string name)
        {
            return new AreaShape(ShapeKind.Rect, new Bounds(Vector2.Zero, Vector2.Zero), false, name, null, Vector2.Zero, 0f, new AreaShape[0]);
        }

        /// <summary>A rect. The game gives the area the rect rounded out to whole 32 cells; <c>Contains</c> tests the true rect, its edges included.</summary>
        [LocalSafe]
        public static AreaShape Rect(Bounds bounds)
        {
            return new AreaShape(ShapeKind.Rect, bounds, true, null, null, bounds.Center, 0f, new AreaShape[0]);
        }

        /// <summary>A polygon. The game gives the area the 32 cells whose centre is inside the polygon, and the cell its centroid is in, so a thin one is not left without cells (a band thinner than a cell may be only that one); <c>Contains</c> tests the true polygon. The centroid of a polygon bent round on itself (a U) can be outside it, and its cell is added all the same. The game is given one cell for each 32 by 32 square inside it, so a very large polygon makes a very large region.</summary>
        [LocalSafe]
        public static AreaShape Polygon(Polygon polygon)
        {
            return new AreaShape(ShapeKind.Polygon, polygon.Bounds, true, null, polygon, polygon.Center, 0f, new AreaShape[0]);
        }

        /// <summary>A circle. The game gives the area the 32 cells whose centre is inside the circle, and always the cell its centre is in, so a small one is that one cell (a radius under 23 at a cell's corner holds no cell's centre); <c>Contains</c> tests the true circle. A radius that is not above 0 throws ArgumentException. The game is given one cell for each 32 by 32 square inside it, so a radius of 4,096 makes about 51,000 cells.</summary>
        [LocalSafe]
        public static AreaShape Circle(Vector2 center, float radius)
        {
            if (!(radius > 0f))
            {
                throw new ArgumentException("A circle needs a radius above 0.");
            }

            var bounds = new Bounds(new Vector2(center.X - radius, center.Y - radius), new Vector2(center.X + radius, center.Y + radius));
            return new AreaShape(ShapeKind.Circle, bounds, true, null, null, center, radius, new AreaShape[0]);
        }

        /// <summary>The shapes together: a point is inside when it is inside any of them. At least one is needed, else ArgumentException.</summary>
        [LocalSafe]
        public static AreaShape Of(params AreaShape[] shapes)
        {
            if (shapes == null || shapes.Length == 0)
            {
                throw new ArgumentException("Of needs at least one shape.");
            }

            var parts = new AreaShape[shapes.Length];
            for (var i = 0; i < shapes.Length; i++)
            {
                if (shapes[i] == null)
                {
                    throw new ArgumentException("Of was given a shape that is null.");
                }

                parts[i] = shapes[i];
            }

            return new AreaShape(ShapeKind.Union, new Bounds(Vector2.Zero, Vector2.Zero), false, null, null, Vector2.Zero, 0f, parts);
        }

        /// <summary>The smallest bounds that cover the whole shape, the true shape and not the cells.</summary>
        [LocalSafe]
        public Bounds Bounds
        {
            get
            {
                if (!resolved)
                {
                    Resolve();
                }

                return bounds;
            }
        }

        /// <summary>Works out the bounds of a region (looked up by name) or of a union (the parts together); nothing is kept when it throws.</summary>
        private void Resolve()
        {
            if (Kind == ShapeKind.Rect)
            {
                if (!AreaSystem.TryRegion(regionName!, out var found))
                {
                    throw new ArgumentException("No World Editor region is named '" + regionName + "'.");
                }

                bounds = found;
                resolved = true;
                return;
            }

            var minX = 0f;
            var minY = 0f;
            var maxX = 0f;
            var maxY = 0f;
            for (var i = 0; i < Parts.Length; i++)
            {
                var b = Parts[i].Bounds;
                if (i == 0)
                {
                    minX = b.Min.X;
                    minY = b.Min.Y;
                    maxX = b.Max.X;
                    maxY = b.Max.Y;
                }
                else
                {
                    minX = MathF.Min(minX, b.Min.X);
                    minY = MathF.Min(minY, b.Min.Y);
                    maxX = MathF.Max(maxX, b.Max.X);
                    maxY = MathF.Max(maxY, b.Max.Y);
                }
            }

            bounds = new Bounds(new Vector2(minX, minY), new Vector2(maxX, maxY));
            resolved = true;
        }

        /// <summary>Whether a point is inside the true shape: a rect's edges count, a polygon as <c>Polygon.Contains</c>, a circle at a distance of at most its radius, and a union when any part has it.</summary>
        [LocalSafe]
        public bool Contains(float x, float y)
        {
            if (Kind == ShapeKind.Rect)
            {
                return x >= Bounds.Min.X && x <= Bounds.Max.X && y >= Bounds.Min.Y && y <= Bounds.Max.Y;
            }

            if (Kind == ShapeKind.Polygon)
            {
                return Corners!.Contains(x, y);
            }

            if (Kind == ShapeKind.Circle)
            {
                var dx = x - Centre.X;
                var dy = y - Centre.Y;
                return dx * dx + dy * dy <= Radius * Radius;
            }

            for (var i = 0; i < Parts.Length; i++)
            {
                if (Parts[i].Contains(x, y))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Whether a point is inside the true shape.</summary>
        [LocalSafe]
        public bool Contains(Vector2 point)
        {
            return Contains(point.X, point.Y);
        }
    }
}
