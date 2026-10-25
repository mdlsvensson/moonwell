using System;

namespace Moonwell
{
    /// <summary>Angles in degrees, as Unit.Facing: 0 along +X, 90 along +Y, counterclockwise. MathF.Sin and its kin take radians; use Angles.Sin and Angles.Cos with degrees.</summary>
    public static class Angles
    {
        private const float RadiansPerDegree = MathF.PI / 180f;
        private const float DegreesPerRadian = 180f / MathF.PI;

        /// <summary>The angle brought into [0, 360), as Facing reads: -90 is 270, 720 is 0.</summary>
        [LocalSafe]
        public static float Normalize(float degrees)
        {
            return Maths.Wrap(degrees, 0f, 360f);
        }

        /// <summary>The shortest signed turn from one angle to another, in [-180, 180): Difference(350, 10) is 20, Difference(10, 350) is -20.</summary>
        [LocalSafe]
        public static float Difference(float from, float to)
        {
            return Maths.Wrap(to - from + 180f, 0f, 360f) - 180f;
        }

        /// <summary>The angle a fraction t of the way from one angle to another, the short way round, normalized: Lerp(350, 10, 0.5) is 0.</summary>
        [LocalSafe]
        public static float Lerp(float from, float to, float t)
        {
            return Normalize(from + Difference(from, to) * t);
        }

        /// <summary>From turned toward to by at most step degrees, the short way round, stopping on to; normalized.</summary>
        [LocalSafe]
        public static float MoveToward(float from, float to, float step)
        {
            var turn = Difference(from, to);
            if (MathF.Abs(turn) <= step)
            {
                return Normalize(to);
            }

            return Normalize(from + Maths.Sign(turn) * step);
        }

        /// <summary>The sine of an angle in degrees.</summary>
        [LocalSafe]
        public static float Sin(float degrees)
        {
            return Trig.Sin(degrees * RadiansPerDegree);
        }

        /// <summary>The cosine of an angle in degrees.</summary>
        [LocalSafe]
        public static float Cos(float degrees)
        {
            return Trig.Cos(degrees * RadiansPerDegree);
        }

        /// <summary>Degrees to radians, for a native that takes radians.</summary>
        [LocalSafe]
        public static float ToRadians(float degrees)
        {
            return degrees * RadiansPerDegree;
        }

        /// <summary>Radians to degrees.</summary>
        [LocalSafe]
        public static float ToDegrees(float radians)
        {
            return radians * DegreesPerRadian;
        }
    }
}
