using System;

namespace Moonwell.Testing
{
    /// <summary>Cases for the maths package's numbers and angles (Maths and Angles), in degrees.</summary>
    public static class ScalarCases
    {
        /// <summary>Lerp does not clamp its t.</summary>
        public static void Lerp(FakeWorld world)
        {
            Expect.Near(12.5f, Maths.Lerp(10f, 20f, 0.25f), "Lerp(10, 20, 0.25)");
            Expect.Near(30f, Maths.Lerp(10f, 20f, 2f), "Lerp(10, 20, 2)");
        }

        /// <summary>InverseLerp gives where a value sits between two, and 0 when the two are the same.</summary>
        public static void InverseLerp(FakeWorld world)
        {
            Expect.Near(0.5f, Maths.InverseLerp(10f, 20f, 15f), "InverseLerp(10, 20, 15)");
            Expect.Near(0f, Maths.InverseLerp(5f, 5f, 5f), "InverseLerp(5, 5, 5)");
        }

        /// <summary>Remap carries a value from one range to another.</summary>
        public static void Remap(FakeWorld world)
        {
            Expect.Near(150f, Maths.Remap(5f, 0f, 10f, 100f, 200f), "Remap(5, 0, 10, 100, 200)");
        }

        /// <summary>Clamp keeps a value in a range; Wrap brings it round into [min, max).</summary>
        public static void ClampAndWrap(FakeWorld world)
        {
            Expect.Near(10f, Maths.Clamp(12f, 0f, 10f), "Clamp(12, 0, 10)");
            Expect.Near(0f, Maths.Clamp(-3f, 0f, 10f), "Clamp(-3, 0, 10)");
            Expect.Near(1f, Maths.Clamp01(1.5f), "Clamp01(1.5)");
            Expect.Near(10f, Maths.Wrap(370f, 0f, 360f), "Wrap(370, 0, 360)");
            Expect.Near(350f, Maths.Wrap(-10f, 0f, 360f), "Wrap(-10, 0, 360)");
            Expect.Near(0f, Maths.Wrap(360f, 0f, 360f), "Wrap(360, 0, 360)");
        }

        /// <summary>MoveToward steps toward a target and stops on it.</summary>
        public static void MoveToward(FakeWorld world)
        {
            Expect.Near(3f, Maths.MoveToward(0f, 10f, 3f), "MoveToward(0, 10, 3)");
            Expect.Near(10f, Maths.MoveToward(9f, 10f, 3f), "MoveToward(9, 10, 3)");
            Expect.Near(7f, Maths.MoveToward(10f, 0f, 3f), "MoveToward(10, 0, 3)");
        }

        /// <summary>Sign gives -1, 0 or 1; Near compares within a tolerance.</summary>
        public static void SignAndNear(FakeWorld world)
        {
            Expect.Near(-1f, Maths.Sign(-2f), "Sign(-2)");
            Expect.Near(0f, Maths.Sign(0f), "Sign(0)");
            Expect.Near(1f, Maths.Sign(0.5f), "Sign(0.5)");
            Expect.True(Maths.Near(1f, 1.0005f), "Near(1, 1.0005)");
            Expect.True(!Maths.Near(1f, 1.01f), "not Near(1, 1.01)");
            Expect.True(Maths.Near(1f, 1.01f, 0.1f), "Near(1, 1.01, 0.1)");
        }

        /// <summary>The float forms: distance, and the facing from one point to another.</summary>
        public static void DistanceAndAngle(FakeWorld world)
        {
            Expect.Near(5f, Maths.Distance(0f, 0f, 3f, 4f), "Distance(0, 0, 3, 4)");
            Expect.Near(25f, Maths.DistanceSquared(0f, 0f, 3f, 4f), "DistanceSquared(0, 0, 3, 4)");
            Expect.Near(90f, Maths.AngleBetween(0f, 0f, 0f, 5f), "AngleBetween to +Y");
            Expect.Near(180f, Maths.AngleBetween(0f, 0f, -1f, 0f), "AngleBetween to -X");
            Expect.Near(270f, Maths.AngleBetween(0f, 0f, 0f, -1f), "AngleBetween to -Y");
            Expect.Near(0f, Maths.AngleBetween(1f, 1f, 1f, 1f), "AngleBetween the same point");
        }

        /// <summary>Polar gives both coordinates of the point that far that way, through out.</summary>
        public static void Polar(FakeWorld world)
        {
            Maths.Polar(0f, 0f, 90f, 10f, out var x, out var y);
            Expect.Near(0f, x, "Polar(0, 0, 90, 10) x");
            Expect.Near(10f, y, "Polar(0, 0, 90, 10) y");
            Maths.Polar(5f, 5f, 180f, 2f, out x, out y);
            Expect.Near(3f, x, "Polar(5, 5, 180, 2) x");
            Expect.Near(5f, y, "Polar(5, 5, 180, 2) y");
        }

        /// <summary>Normalize brings any angle into [0, 360), negative and large ones too.</summary>
        public static void AnglesNormalize(FakeWorld world)
        {
            Expect.Near(270f, Angles.Normalize(-90f), "Normalize(-90)");
            Expect.Near(0f, Angles.Normalize(720f), "Normalize(720)");
            Expect.Near(0f, Angles.Normalize(360f), "Normalize(360)");
            Expect.Near(45f, Angles.Normalize(45f), "Normalize(45)");
            Expect.Near(350f, Angles.Normalize(-370f), "Normalize(-370)");
        }

        /// <summary>Difference is the shortest signed turn, in [-180, 180).</summary>
        public static void AnglesDifference(FakeWorld world)
        {
            Expect.Near(20f, Angles.Difference(350f, 10f), "Difference(350, 10)");
            Expect.Near(-20f, Angles.Difference(10f, 350f), "Difference(10, 350)");
            Expect.Near(-180f, Angles.Difference(0f, 180f), "Difference(0, 180)");
            Expect.Near(90f, Angles.Difference(-90f, 0f), "Difference(-90, 0)");
        }

        /// <summary>Lerp and MoveToward on angles go the short way round, and the result is normalized.</summary>
        public static void AnglesLerpAndMove(FakeWorld world)
        {
            Expect.Near(0f, Angles.Lerp(350f, 10f, 0.5f), "Lerp(350, 10, 0.5)");
            Expect.Near(355f, Angles.MoveToward(350f, 10f, 5f), "MoveToward(350, 10, 5)");
            Expect.Near(10f, Angles.MoveToward(350f, 10f, 50f), "MoveToward(350, 10, 50) stops on it");
            Expect.Near(340f, Angles.MoveToward(10f, 340f, 40f), "MoveToward(10, 340, 40)");
        }

        /// <summary>Sin and Cos take degrees; the conversions agree with MathF.PI.</summary>
        public static void AnglesTrig(FakeWorld world)
        {
            Expect.Near(1f, Angles.Sin(90f), "Sin(90)");
            Expect.Near(-1f, Angles.Cos(180f), "Cos(180)");
            Expect.Near(0.5f, Angles.Sin(30f), "Sin(30)");
            Expect.Near(MathF.PI, Angles.ToRadians(180f), "ToRadians(180)");
            Expect.Near(90f, Angles.ToDegrees(MathF.PI / 2f), "ToDegrees(pi / 2)");
        }
    }
}
