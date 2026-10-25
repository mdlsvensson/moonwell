namespace Moonwell.Testing
{
    /// <summary>Cases for Vector2, in degrees, with Godot's names where the idea is the same.</summary>
    public static class Vector2Cases
    {
        /// <summary>Fails unless the two vectors are within 0.001 on each axis.</summary>
        internal static void Near(Vector2 expected, Vector2 actual, string what)
        {
            Expect.Near(expected.X, actual.X, what + " x");
            Expect.Near(expected.Y, actual.Y, what + " y");
        }

        /// <summary>Fails if either component is NaN.</summary>
        internal static void NotNaN(Vector2 v, string what)
        {
            Expect.True(!float.IsNaN(v.X) && !float.IsNaN(v.Y), what + " is not NaN");
        }

        /// <summary>Length, its square, and a normalized vector; zero normalizes to zero.</summary>
        public static void Basics(FakeWorld world)
        {
            var v = new Vector2(3f, 4f);
            Expect.Near(5f, v.Length, "(3, 4).Length");
            Expect.Near(25f, v.LengthSquared, "(3, 4).LengthSquared");
            Near(new Vector2(0.6f, 0.8f), v.Normalized(), "(3, 4).Normalized()");
            Near(Vector2.Zero, Vector2.Zero.Normalized(), "Zero.Normalized()");
            NotNaN(Vector2.Zero.Normalized(), "Zero.Normalized()");
            Near(new Vector2(1f, 1f), Vector2.One, "One");
            Near(new Vector2(1f, 0f), Vector2.Right, "Right");
            Near(new Vector2(0f, 1f), Vector2.Up, "Up");
            Expect.Near(5f, v.DistanceTo(Vector2.Zero), "DistanceTo");
            Expect.Near(25f, v.DistanceSquaredTo(Vector2.Zero), "DistanceSquaredTo");
        }

        /// <summary>A vector's direction and the facing from one point to another, 0 to 360.</summary>
        public static void Angles(FakeWorld world)
        {
            Expect.Near(90f, Vector2.Up.Angle, "Up.Angle");
            Expect.Near(180f, new Vector2(-1f, 0f).Angle, "(-1, 0).Angle");
            Expect.Near(270f, new Vector2(0f, -1f).Angle, "(0, -1).Angle");
            Expect.Near(0f, Vector2.Zero.Angle, "Zero.Angle");
            Expect.Near(90f, Vector2.Zero.AngleTo(new Vector2(0f, 5f)), "AngleTo a point above");
            var p = new Vector2(7f, -3f);
            Expect.Near(0f, p.AngleTo(p), "AngleTo the same point");
        }

        /// <summary>Offset, Rotated and FromAngle take degrees.</summary>
        public static void OffsetAndRotate(FakeWorld world)
        {
            Near(new Vector2(0f, 10f), Vector2.Zero.Offset(90f, 10f), "Zero.Offset(90, 10)");
            Near(new Vector2(5f, 3f), new Vector2(5f, 5f).Offset(270f, 2f), "(5, 5).Offset(270, 2)");
            Near(Vector2.Up, Vector2.Right.Rotated(90f), "Right.Rotated(90)");
            Near(new Vector2(-2f, 0f), Vector2.FromAngle(180f, 2f), "FromAngle(180, 2)");
            Near(Vector2.Up, Vector2.FromAngle(90f), "FromAngle(90)");
        }

        /// <summary>Dot and the 2D cross product, a float.</summary>
        public static void Products(FakeWorld world)
        {
            Expect.Near(0f, Vector2.Right.Dot(Vector2.Up), "Right.Dot(Up)");
            Expect.Near(11f, new Vector2(1f, 2f).Dot(new Vector2(3f, 4f)), "(1, 2).Dot(3, 4)");
            Expect.Near(1f, Vector2.Right.Cross(Vector2.Up), "Right.Cross(Up)");
            Expect.Near(-1f, Vector2.Up.Cross(Vector2.Right), "Up.Cross(Right)");
        }

        /// <summary>Bounce is the physical bounce off a surface; Reflect mirrors across the normal; Slide keeps the part along the surface.</summary>
        public static void BounceReflectSlide(FakeWorld world)
        {
            var v = new Vector2(1f, -1f);
            Near(new Vector2(1f, 1f), v.Bounce(Vector2.Up), "(1, -1).Bounce(Up)");
            Near(new Vector2(-1f, -1f), v.Reflect(Vector2.Up), "(1, -1).Reflect(Up)");
            Near(new Vector2(1f, 0f), v.Slide(Vector2.Up), "(1, -1).Slide(Up)");
        }

        /// <summary>LimitLength, Project and MoveToward, with zero vectors giving zero.</summary>
        public static void Limits(FakeWorld world)
        {
            Near(new Vector2(3f, 0f), new Vector2(10f, 0f).LimitLength(3f), "(10, 0).LimitLength(3)");
            Near(new Vector2(1f, 0f), new Vector2(1f, 0f).LimitLength(3f), "(1, 0).LimitLength(3)");
            Near(Vector2.Zero, Vector2.Zero.LimitLength(3f), "Zero.LimitLength(3)");
            NotNaN(Vector2.Zero.LimitLength(3f), "Zero.LimitLength(3)");
            Near(new Vector2(2f, 0f), new Vector2(2f, 2f).Project(Vector2.Right), "(2, 2).Project(Right)");
            Near(Vector2.Zero, new Vector2(1f, 1f).Project(Vector2.Zero), "(1, 1).Project(Zero)");
            NotNaN(new Vector2(1f, 1f).Project(Vector2.Zero), "(1, 1).Project(Zero)");
            Near(new Vector2(3f, 0f), Vector2.Zero.MoveToward(new Vector2(10f, 0f), 3f), "Zero.MoveToward((10, 0), 3)");
            Near(new Vector2(10f, 0f), new Vector2(9f, 0f).MoveToward(new Vector2(10f, 0f), 3f), "(9, 0).MoveToward((10, 0), 3)");
            Near(new Vector2(4f, 4f), new Vector2(4f, 4f).MoveToward(new Vector2(4f, 4f), 3f), "MoveToward its own point");
        }

        /// <summary>Lerp is a fraction of the way between two points.</summary>
        public static void Lerp(FakeWorld world)
        {
            Near(new Vector2(5f, 10f), Vector2.Lerp(Vector2.Zero, new Vector2(10f, 20f), 0.5f), "Lerp(Zero, (10, 20), 0.5)");
        }

        /// <summary>The arithmetic operators.</summary>
        public static void Operators(FakeWorld world)
        {
            Near(new Vector2(4f, 6f), new Vector2(1f, 2f) + new Vector2(3f, 4f), "(1, 2) + (3, 4)");
            Near(new Vector2(-2f, -2f), new Vector2(1f, 2f) - new Vector2(3f, 4f), "(1, 2) - (3, 4)");
            Near(new Vector2(-1f, -2f), -new Vector2(1f, 2f), "-(1, 2)");
            Near(new Vector2(2f, 4f), 2f * new Vector2(1f, 2f), "2 * (1, 2)");
            Near(new Vector2(2f, 4f), new Vector2(1f, 2f) * 2f, "(1, 2) * 2");
            Near(new Vector2(1f, 2f), new Vector2(2f, 4f) / 2f, "(2, 4) / 2");
        }

        /// <summary>==, Equals and the hash code compare the components.</summary>
        public static void Equality(FakeWorld world)
        {
            var a = new Vector2(1f, 2f);
            Expect.True(a == new Vector2(1f, 2f), "(1, 2) == (1, 2)");
            Expect.True(a != new Vector2(2f, 1f), "(1, 2) != (2, 1)");
            Expect.True(a.Equals((object)new Vector2(1f, 2f)), "Equals(object)");
            Expect.True(a.Equals(new Vector2(1f, 2f)), "Equals(Vector2)");
            Expect.True(!a.Equals("x"), "not Equals(\"x\")");
            Expect.Equal(a.GetHashCode(), new Vector2(1f, 2f).GetHashCode(), "equal values, equal hash codes");
            Expect.True(a.Near(new Vector2(1.0005f, 2f)), "Near within 0.001");
            Expect.True(!a.Near(new Vector2(1.1f, 2f)), "not Near at 0.1");
        }

        /// <summary>ToString writes whole floats without a point.</summary>
        public static void Text(FakeWorld world)
        {
            Expect.Equal("(12.5, 40)", new Vector2(12.5f, 40f).ToString(), "(12.5, 40)");
            Expect.Equal("(0, -1)", new Vector2(0f, -1f).ToString(), "(0, -1)");
        }
    }
}
