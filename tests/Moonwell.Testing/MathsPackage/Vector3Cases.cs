namespace Moonwell.Testing
{
    /// <summary>Cases for Vector3 (Z is height) and the easing curves.</summary>
    public static class Vector3Cases
    {
        /// <summary>Fails unless the two vectors are within 0.001 on each axis.</summary>
        internal static void Near(Vector3 expected, Vector3 actual, string what)
        {
            Expect.Near(expected.X, actual.X, what + " x");
            Expect.Near(expected.Y, actual.Y, what + " y");
            Expect.Near(expected.Z, actual.Z, what + " z");
        }

        /// <summary>Length, normalizing, distance, and zero normalizing to zero.</summary>
        public static void Basics(FakeWorld world)
        {
            var v = new Vector3(2f, 3f, 6f);
            Expect.Near(7f, v.Length, "(2, 3, 6).Length");
            Expect.Near(49f, v.LengthSquared, "(2, 3, 6).LengthSquared");
            Near(new Vector3(2f / 7f, 3f / 7f, 6f / 7f), v.Normalized(), "(2, 3, 6).Normalized()");
            Near(Vector3.Zero, Vector3.Zero.Normalized(), "Zero.Normalized()");
            Expect.True(!float.IsNaN(Vector3.Zero.Normalized().X), "Zero.Normalized() is not NaN");
            Expect.Near(7f, v.DistanceTo(Vector3.Zero), "DistanceTo");
            Expect.Near(49f, v.DistanceSquaredTo(Vector3.Zero), "DistanceSquaredTo");
            Near(new Vector3(1f, 1f, 1f), Vector3.One, "One");
            Expect.Near(32f, new Vector3(1f, 2f, 3f).Dot(new Vector3(4f, 5f, 6f)), "Dot");
        }

        /// <summary>Yaw is the XY direction, Pitch the tilt up from it, both in degrees; FromAngles makes a vector from them.</summary>
        public static void YawPitch(FakeWorld world)
        {
            Expect.Near(90f, new Vector3(0f, 1f, 0f).Yaw, "(0, 1, 0).Yaw");
            Expect.Near(45f, new Vector3(1f, 0f, 1f).Pitch, "(1, 0, 1).Pitch");
            Expect.Near(90f, Vector3.UnitZ.Pitch, "UnitZ.Pitch");
            Expect.Near(-90f, (-Vector3.UnitZ).Pitch, "-UnitZ.Pitch");
            Expect.Near(0f, Vector3.Zero.Pitch, "Zero.Pitch");
            Expect.Near(0f, Vector3.Zero.Yaw, "Zero.Yaw");
            Near(Vector3.UnitY, Vector3.FromAngles(90f, 0f), "FromAngles(90, 0)");
            Near(Vector3.UnitZ, Vector3.FromAngles(0f, 90f), "FromAngles(0, 90)");
            Near(new Vector3(-2f, 0f, 0f), Vector3.FromAngles(180f, 0f, 2f), "FromAngles(180, 0, 2)");
        }

        /// <summary>The cross product follows the right hand: X cross Y is Z.</summary>
        public static void Cross(FakeWorld world)
        {
            Near(Vector3.UnitZ, Vector3.UnitX.Cross(Vector3.UnitY), "UnitX.Cross(UnitY)");
            Near(-Vector3.UnitZ, Vector3.UnitY.Cross(Vector3.UnitX), "UnitY.Cross(UnitX)");
        }

        /// <summary>Bounce, Reflect and Slide off a surface, as Vector2's.</summary>
        public static void Bounce(FakeWorld world)
        {
            var v = new Vector3(1f, 0f, -1f);
            Near(new Vector3(1f, 0f, 1f), v.Bounce(Vector3.UnitZ), "(1, 0, -1).Bounce(UnitZ)");
            Near(new Vector3(-1f, 0f, -1f), v.Reflect(Vector3.UnitZ), "(1, 0, -1).Reflect(UnitZ)");
            Near(new Vector3(1f, 0f, 0f), v.Slide(Vector3.UnitZ), "(1, 0, -1).Slide(UnitZ)");
            Near(new Vector3(2f, 0f, 0f), new Vector3(2f, 2f, 2f).Project(Vector3.UnitX), "(2, 2, 2).Project(UnitX)");
            Near(Vector3.Zero, new Vector3(1f, 1f, 1f).Project(Vector3.Zero), "Project(Zero)");
            Near(new Vector3(3f, 0f, 0f), new Vector3(10f, 0f, 0f).LimitLength(3f), "LimitLength(3)");
            Near(Vector3.Zero, Vector3.Zero.LimitLength(3f), "Zero.LimitLength(3)");
            Near(new Vector3(0f, 0f, 3f), Vector3.Zero.MoveToward(new Vector3(0f, 0f, 10f), 3f), "MoveToward");
            Near(new Vector3(5f, 5f, 5f), Vector3.Lerp(Vector3.Zero, new Vector3(10f, 10f, 10f), 0.5f), "Lerp");
        }

        /// <summary>A Vector3 from a Vector2 and a height, and back.</summary>
        public static void FromVector2(FakeWorld world)
        {
            var v = new Vector3(new Vector2(1f, 2f), 3f);
            Expect.True(v.XY == new Vector2(1f, 2f), "XY");
            Expect.Near(3f, v.Z, "Z");
        }

        /// <summary>Operators, equality and text.</summary>
        public static void EqualityAndText(FakeWorld world)
        {
            var a = new Vector3(1f, 2.5f, 0f);
            Expect.True(a == new Vector3(1f, 2.5f, 0f), "==");
            Expect.True(a != new Vector3(1f, 2.5f, 1f), "!=");
            Expect.True(a.Equals((object)new Vector3(1f, 2.5f, 0f)), "Equals(object)");
            Expect.True(!a.Equals("x"), "not Equals(\"x\")");
            Expect.Equal(a.GetHashCode(), new Vector3(1f, 2.5f, 0f).GetHashCode(), "equal values, equal hash codes");
            Expect.True(a.Near(new Vector3(1.0005f, 2.5f, 0f)), "Near");
            Expect.Equal("(1, 2.5, 0)", a.ToString(), "ToString");
            Near(new Vector3(2f, 5f, 0f), a * 2f, "a * 2");
            Near(new Vector3(2f, 5f, 0f), 2f * a, "2 * a");
            Near(new Vector3(0.5f, 1.25f, 0f), a / 2f, "a / 2");
            Near(new Vector3(2f, 3.5f, 1f), a + Vector3.One, "a + One");
            Near(new Vector3(0f, 1.5f, -1f), a - Vector3.One, "a - One");
        }

        /// <summary>Every curve gives 0 at 0 and 1 at 1.</summary>
        public static void EaseEnds(FakeWorld world)
        {
            var curves = new System.Func<float, float>[]
            {
                Ease.Linear, Ease.SmoothStep, Ease.InQuad, Ease.OutQuad, Ease.InOutQuad, Ease.InCubic, Ease.OutCubic,
                Ease.InOutCubic, Ease.InSine, Ease.OutSine, Ease.InOutSine, Ease.InBack, Ease.OutBack, Ease.InOutBack,
                Ease.InBounce, Ease.OutBounce, Ease.InOutBounce,
            };
            for (var i = 0; i < curves.Length; i++)
            {
                Expect.Near(0f, curves[i](0f), "curve " + i + " at 0");
                Expect.Near(1f, curves[i](1f), "curve " + i + " at 1");
            }
        }

        /// <summary>Values at the middle.</summary>
        public static void EaseMiddles(FakeWorld world)
        {
            Expect.Near(0.75f, Ease.OutQuad(0.5f), "OutQuad(0.5)");
            Expect.Near(0.25f, Ease.InQuad(0.5f), "InQuad(0.5)");
            Expect.Near(0.5f, Ease.SmoothStep(0.5f), "SmoothStep(0.5)");
            Expect.Near(0.5f, Ease.InOutCubic(0.5f), "InOutCubic(0.5)");
            Expect.Near(0.5f, Ease.Linear(0.5f), "Linear(0.5)");
            Expect.Near(0.125f, Ease.InCubic(0.5f), "InCubic(0.5)");
        }

        /// <summary>t is clamped to 0 to 1 first; Back overshoots past 1.</summary>
        public static void EaseClampedAndOvershoot(FakeWorld world)
        {
            Expect.Near(1f, Ease.OutQuad(2f), "OutQuad(2)");
            Expect.Near(0f, Ease.InQuad(-1f), "InQuad(-1)");
            Expect.True(Ease.OutBack(0.5f) > 1f, "OutBack(0.5) > 1");
            Expect.True(Ease.InBack(0.5f) < 0f, "InBack(0.5) < 0");
        }
    }
}
