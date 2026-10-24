using System;

namespace Moonwell
{
    /// <summary>Numbers: interpolation, clamping and wrapping, and the float forms of distance and direction that code run every tick uses instead of vectors, since a vector is a table to make. Angles are in degrees.</summary>
    public static class Maths
    {
        /// <summary>The value a fraction t of the way from a to b: 0 gives a, 1 gives b. t is not clamped, so 2 goes past b.</summary>
        [LocalSafe]
        public static float Lerp(float a, float b, float t)
        {
            return a + (b - a) * t;
        }

        /// <summary>Where value sits between a and b, as Lerp's t: a gives 0, b gives 1. Not clamped; 0 when a and b are the same.</summary>
        [LocalSafe]
        public static float InverseLerp(float a, float b, float value)
        {
            if (a == b)
            {
                return 0f;
            }

            return (value - a) / (b - a);
        }

        /// <summary>Carries a value from one range to another: Remap(5, 0, 10, 100, 200) is 150. Not clamped.</summary>
        [LocalSafe]
        public static float Remap(float value, float fromMin, float fromMax, float toMin, float toMax)
        {
            return Lerp(toMin, toMax, InverseLerp(fromMin, fromMax, value));
        }

        /// <summary>The value kept within min and max.</summary>
        [LocalSafe]
        public static float Clamp(float value, float min, float max)
        {
            if (value < min)
            {
                return min;
            }

            if (value > max)
            {
                return max;
            }

            return value;
        }

        /// <summary>The value kept within 0 and 1.</summary>
        [LocalSafe]
        public static float Clamp01(float value)
        {
            return Clamp(value, 0f, 1f);
        }

        /// <summary>The value brought round into [min, max), as a clock does: Wrap(370, 0, 360) is 10, Wrap(-10, 0, 360) is 350.</summary>
        [LocalSafe]
        public static float Wrap(float value, float min, float max)
        {
            var range = max - min;
            if (range <= 0f)
            {
                return min;
            }

            var wrapped = value - range * MathF.Floor((value - min) / range);
            // A value just below min comes out as max after rounding; that is min.
            return wrapped >= max ? min : wrapped;
        }

        /// <summary>From moved toward to by at most step, stopping on to.</summary>
        [LocalSafe]
        public static float MoveToward(float from, float to, float step)
        {
            var gap = to - from;
            if (MathF.Abs(gap) <= step)
            {
                return to;
            }

            return from + Sign(gap) * step;
        }

        /// <summary>Whether a and b are within tolerance of each other. Use it on computed floats, where == is exact and rarely holds.</summary>
        [LocalSafe]
        public static bool Near(float a, float b, float tolerance = 0.001f)
        {
            return MathF.Abs(a - b) <= tolerance;
        }

        /// <summary>-1 for a negative value, 1 for a positive one, 0 for 0.</summary>
        [LocalSafe]
        public static float Sign(float value)
        {
            if (value > 0f)
            {
                return 1f;
            }

            if (value < 0f)
            {
                return -1f;
            }

            return 0f;
        }

        /// <summary>The distance between two points, with no vector made.</summary>
        [LocalSafe]
        public static float Distance(float x1, float y1, float x2, float y2)
        {
            return Trig.Sqrt(DistanceSquared(x1, y1, x2, y2));
        }

        /// <summary>The distance between two points squared: cheaper than Distance, and enough to compare distances.</summary>
        [LocalSafe]
        public static float DistanceSquared(float x1, float y1, float x2, float y2)
        {
            var dx = x2 - x1;
            var dy = y2 - y1;
            return dx * dx + dy * dy;
        }

        /// <summary>The facing in degrees from the first point to the second, 0 to 360 (0 along +X, 90 along +Y); 0 for the same point.</summary>
        [LocalSafe]
        public static float AngleBetween(float x1, float y1, float x2, float y2)
        {
            var dx = x2 - x1;
            var dy = y2 - y1;
            if (dx == 0f && dy == 0f)
            {
                return 0f;
            }

            return Angles.Normalize(Angles.ToDegrees(Trig.Atan2(dy, dx)));
        }

        /// <summary>The point a distance from (x, y) in the direction of an angle in degrees, given through x2 and y2, with no vector made.</summary>
        [LocalSafe]
        public static void Polar(float x, float y, float degrees, float distance, out float x2, out float y2)
        {
            x2 = x + distance * Angles.Cos(degrees);
            y2 = y + distance * Angles.Sin(degrees);
        }
    }
}
