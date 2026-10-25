using System;

namespace Moonwell.Testing
{
    /// <summary>Cases for Bounds and Polygon.</summary>
    public static class ShapeCases
    {
        /// <summary>The L shape the cases share: a 10 by 10 square with its top-right 6 by 6 cut out.</summary>
        internal static Polygon LShape()
        {
            return new Polygon(
                new Vector2(0f, 0f), new Vector2(10f, 0f), new Vector2(10f, 4f),
                new Vector2(4f, 4f), new Vector2(4f, 10f), new Vector2(0f, 10f));
        }

        /// <summary>Any two opposite corners make the same bounds.</summary>
        public static void BoundsCorners(FakeWorld world)
        {
            var b = new Bounds(new Vector2(10f, 10f), new Vector2(0f, 0f));
            Expect.True(b.Min == new Vector2(0f, 0f), "Min");
            Expect.True(b.Max == new Vector2(10f, 10f), "Max");
            Expect.True(b.Center == new Vector2(5f, 5f), "Center");
            Expect.True(b.Size == new Vector2(10f, 10f), "Size");
            var c = Bounds.FromCenter(new Vector2(5f, 5f), new Vector2(10f, 4f));
            Expect.True(c.Min == new Vector2(0f, 3f) && c.Max == new Vector2(10f, 7f), "FromCenter");
            Expect.True(b == new Bounds(new Vector2(0f, 0f), new Vector2(10f, 10f)), "==");
            Expect.Equal("((0, 0), (10, 10))", b.ToString(), "ToString");
        }

        /// <summary>Contains counts the edges as inside.</summary>
        public static void BoundsContains(FakeWorld world)
        {
            var b = new Bounds(Vector2.Zero, new Vector2(10f, 10f));
            Expect.True(b.Contains(new Vector2(5f, 5f)), "(5, 5)");
            Expect.True(b.Contains(new Vector2(10f, 10f)), "(10, 10) on the corner");
            Expect.True(!b.Contains(new Vector2(11f, 5f)), "(11, 5)");
        }

        /// <summary>Touching bounds intersect; apart ones do not.</summary>
        public static void BoundsIntersects(FakeWorld world)
        {
            var b = new Bounds(Vector2.Zero, new Vector2(10f, 10f));
            Expect.True(b.Intersects(new Bounds(new Vector2(10f, 0f), new Vector2(20f, 10f))), "touching at x 10");
            Expect.True(b.Intersects(new Bounds(new Vector2(5f, 5f), new Vector2(15f, 15f))), "overlapping");
            Expect.True(!b.Intersects(new Bounds(new Vector2(11f, 0f), new Vector2(20f, 10f))), "apart");
        }

        /// <summary>Clamp gives the nearest point inside; a negative Expanded shrinks, never past a zero size at the center.</summary>
        public static void BoundsClampExpand(FakeWorld world)
        {
            var b = new Bounds(Vector2.Zero, new Vector2(10f, 10f));
            Expect.True(b.Clamp(new Vector2(20f, 5f)) == new Vector2(10f, 5f), "Clamp((20, 5))");
            Expect.True(b.Clamp(new Vector2(3f, 4f)) == new Vector2(3f, 4f), "Clamp inside");
            var grown = b.Expanded(2f);
            Expect.True(grown.Min == new Vector2(-2f, -2f) && grown.Max == new Vector2(12f, 12f), "Expanded(2)");
            var shrunk = b.Expanded(-6f);
            Expect.True(shrunk.Min == new Vector2(5f, 5f) && shrunk.Max == new Vector2(5f, 5f), "Expanded(-6)");
        }

        /// <summary>Contains on an L shape: the notch is out, the arms in, an edge in.</summary>
        public static void PolygonContains(FakeWorld world)
        {
            var l = LShape();
            Expect.True(!l.Contains(new Vector2(7f, 7f)), "the notch");
            Expect.True(l.Contains(new Vector2(7f, 2f)), "the bottom arm");
            Expect.True(l.Contains(new Vector2(2f, 7f)), "the left arm");
            Expect.True(l.Contains(new Vector2(5f, 0f)), "on the bottom edge");
            Expect.True(l.Contains(new Vector2(4f, 7f)), "on the notch's edge");
            Expect.True(!l.Contains(new Vector2(-1f, 5f)), "left of it");
        }

        /// <summary>Contains on floats answers as it does on a vector, for the L shape's notch, arms and edges.</summary>
        public static void PolygonContainsFloats(FakeWorld world)
        {
            var l = LShape();
            Expect.True(!l.Contains(7f, 7f), "the notch");
            Expect.True(l.Contains(7f, 2f), "the bottom arm");
            Expect.True(l.Contains(2f, 7f), "the left arm");
            Expect.True(l.Contains(5f, 0f), "on the bottom edge");
            Expect.True(l.Contains(4f, 7f), "on the notch's edge");
            Expect.True(!l.Contains(-1f, 5f), "left of it");
            Expect.True(!l.Contains(5f, 11f), "above it");
            Expect.True(l.Contains(0f, 0f), "on a corner");
        }

        /// <summary>Area is positive in either winding; Center is the area's centroid; Bounds holds every point.</summary>
        public static void PolygonAreaCenter(FakeWorld world)
        {
            var counter = new Polygon(new Vector2(0f, 0f), new Vector2(10f, 0f), new Vector2(10f, 10f), new Vector2(0f, 10f));
            var clockwise = new Polygon(new Vector2(0f, 0f), new Vector2(0f, 10f), new Vector2(10f, 10f), new Vector2(10f, 0f));
            Expect.Near(100f, counter.Area, "counterclockwise area");
            Expect.Near(100f, clockwise.Area, "clockwise area");
            Vector2Cases.Near(new Vector2(5f, 5f), counter.Center, "counterclockwise center");
            Vector2Cases.Near(new Vector2(5f, 5f), clockwise.Center, "clockwise center");
            Expect.Near(64f, LShape().Area, "L area");
            Expect.True(LShape().Bounds == new Bounds(Vector2.Zero, new Vector2(10f, 10f)), "L bounds");
            Expect.Equal(6, LShape().Points.Count, "L points");
        }

        /// <summary>A polygon keeps its own copy of the points it was given.</summary>
        public static void PolygonCopies(FakeWorld world)
        {
            var points = new[] { new Vector2(0f, 0f), new Vector2(10f, 0f), new Vector2(10f, 10f) };
            var polygon = new Polygon(points);
            points[0] = new Vector2(99f, 99f);
            Expect.True(polygon.Points[0] == new Vector2(0f, 0f), "Points[0] unchanged");
        }

        /// <summary>Fewer than three points is an error.</summary>
        public static void PolygonTooFew(FakeWorld world)
        {
            Expect.Throws<ArgumentException>(() => new Polygon(new Vector2(0f, 0f), new Vector2(1f, 0f)), "two points");
        }
    }
}
