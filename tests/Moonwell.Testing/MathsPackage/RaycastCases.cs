using System;

namespace Moonwell.Testing
{
    /// <summary>Cases for Ray, RayHit and Raycast: the first hit along a segment, with its point, normal and fraction.</summary>
    public static class RaycastCases
    {
        private static void Hit(RayHit hit, Vector2 point, Vector2 normal, float fraction, string what)
        {
            Vector2Cases.Near(point, hit.Point, what + " point");
            Vector2Cases.Near(normal, hit.Normal, what + " normal");
            Expect.Near(fraction, hit.Fraction, what + " fraction");
        }

        /// <summary>A segment through a circle hits its near side.</summary>
        public static void CircleHit(FakeWorld world)
        {
            Expect.True(Raycast.Circle(new Vector2(-10f, 0f), new Vector2(10f, 0f), Vector2.Zero, 2f, out var hit), "hits");
            Hit(hit, new Vector2(-2f, 0f), new Vector2(-1f, 0f), 0.4f, "circle");
        }

        /// <summary>A segment passing beside the circle misses, and the hit is default.</summary>
        public static void CircleMiss(FakeWorld world)
        {
            Expect.True(!Raycast.Circle(new Vector2(-10f, 3f), new Vector2(10f, 3f), Vector2.Zero, 2f, out var hit), "misses");
            Expect.True(hit.Point == Vector2.Zero && hit.Fraction == 0f, "default hit");
        }

        /// <summary>A segment that ends before the circle misses.</summary>
        public static void CircleShort(FakeWorld world)
        {
            Expect.True(!Raycast.Circle(new Vector2(-10f, 0f), new Vector2(-5f, 0f), Vector2.Zero, 2f, out _), "ends short");
        }

        /// <summary>Starting inside is a hit at 0, with the normal against the segment's direction.</summary>
        public static void CircleInside(FakeWorld world)
        {
            Expect.True(Raycast.Circle(Vector2.Zero, new Vector2(10f, 0f), Vector2.Zero, 2f, out var hit), "hits");
            Hit(hit, Vector2.Zero, new Vector2(-1f, 0f), 0f, "inside");
        }

        /// <summary>A zero-length segment hits only inside, with a zero normal.</summary>
        public static void CircleZeroLength(FakeWorld world)
        {
            Expect.True(Raycast.Circle(new Vector2(1f, 0f), new Vector2(1f, 0f), Vector2.Zero, 2f, out var hit), "inside hits");
            Hit(hit, new Vector2(1f, 0f), Vector2.Zero, 0f, "zero length inside");
            Expect.True(!Raycast.Circle(new Vector2(5f, 0f), new Vector2(5f, 0f), Vector2.Zero, 2f, out _), "outside misses");
        }

        /// <summary>A segment through a wall hits it, with the wall's normal facing back toward the start.</summary>
        public static void SegmentHit(FakeWorld world)
        {
            var a = new Vector2(0f, -5f);
            var b = new Vector2(0f, 5f);
            Expect.True(Raycast.Segment(new Vector2(-10f, 0f), new Vector2(10f, 0f), a, b, out var hit), "hits");
            Hit(hit, Vector2.Zero, new Vector2(-1f, 0f), 0.5f, "wall from the left");
            Expect.True(Raycast.Segment(new Vector2(10f, 0f), new Vector2(-10f, 0f), a, b, out hit), "hits from the right");
            Hit(hit, Vector2.Zero, new Vector2(1f, 0f), 0.5f, "wall from the right");
        }

        /// <summary>A parallel segment misses, collinear too.</summary>
        public static void SegmentParallel(FakeWorld world)
        {
            Expect.True(!Raycast.Segment(new Vector2(-10f, 1f), new Vector2(10f, 1f), new Vector2(-5f, 0f), new Vector2(5f, 0f), out _), "parallel");
            Expect.True(!Raycast.Segment(new Vector2(-10f, 0f), new Vector2(10f, 0f), new Vector2(-5f, 0f), new Vector2(5f, 0f), out _), "collinear");
        }

        /// <summary>A segment into bounds hits the near side.</summary>
        public static void BoundsHit(FakeWorld world)
        {
            var box = new Bounds(Vector2.Zero, new Vector2(10f, 10f));
            Expect.True(Raycast.Bounds(new Vector2(-5f, 5f), new Vector2(5f, 5f), box, out var hit), "hits");
            Hit(hit, new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f, "bounds");
            Expect.True(Raycast.Bounds(new Vector2(5f, 5f), new Vector2(20f, 5f), box, out hit), "starts inside");
            Hit(hit, new Vector2(5f, 5f), new Vector2(-1f, 0f), 0f, "bounds inside");
            Expect.True(!Raycast.Bounds(new Vector2(-5f, 20f), new Vector2(5f, 20f), box, out _), "above misses");
        }

        /// <summary>A segment into a square polygon hits as the bounds do.</summary>
        public static void PolygonHit(FakeWorld world)
        {
            var square = new Polygon(new Vector2(0f, 0f), new Vector2(10f, 0f), new Vector2(10f, 10f), new Vector2(0f, 10f));
            Expect.True(Raycast.Polygon(new Vector2(-5f, 5f), new Vector2(5f, 5f), square, out var hit), "hits");
            Hit(hit, new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f, "square");
            Expect.True(!Raycast.Polygon(new Vector2(-5f, 20f), new Vector2(5f, 20f), square, out _), "above misses");
        }

        /// <summary>Through the L shape's notch from above, the first edge is the notch's floor, not the bottom.</summary>
        public static void PolygonFirstEdge(FakeWorld world)
        {
            var l = ShapeCases.LShape();
            Expect.True(Raycast.Polygon(new Vector2(7f, 20f), new Vector2(7f, -20f), l, out var hit), "hits");
            Hit(hit, new Vector2(7f, 4f), new Vector2(0f, 1f), 0.4f, "notch floor");
            Expect.True(!Raycast.Polygon(new Vector2(7f, 20f), new Vector2(7f, 6f), l, out _), "stops in the notch");
        }

        /// <summary>A ray's point at a distance; a zero direction is an error.</summary>
        public static void RayAt(FakeWorld world)
        {
            var ray = new Ray(Vector2.Zero, new Vector2(0f, 5f));
            Vector2Cases.Near(Vector2.Up, ray.Direction, "Direction normalized");
            Vector2Cases.Near(new Vector2(0f, 3f), ray.At(3f), "At(3)");
            Expect.Throws<ArgumentException>(() => new Ray(Vector2.Zero, Vector2.Zero), "zero direction");
        }
    }
}
