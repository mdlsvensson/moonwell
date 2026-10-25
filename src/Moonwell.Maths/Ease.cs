using System;

namespace Moonwell
{
    /// <summary>
    /// Easing curves, for a movement that speeds up or slows down: each takes how far along it is (0 to 1, clamped) and gives
    /// how far to be (0 at 0, 1 at 1). Use one as Lerp's t: <c>Vector2.Lerp(start, end, Ease.OutQuad(t))</c>. In starts
    /// slow, Out ends slow, InOut does both. The curves are the usual ones (easings.net).
    /// </summary>
    public static class Ease
    {
        private const float Back = 1.70158f;
        private const float BackInOut = Back * 1.525f;
        private const float HalfPi = MathF.PI / 2f;

        /// <summary>No easing: t itself.</summary>
        [LocalSafe]
        public static float Linear(float t)
        {
            return Maths.Clamp01(t);
        }

        /// <summary>Slow at both ends, a gentler InOut.</summary>
        [LocalSafe]
        public static float SmoothStep(float t)
        {
            t = Maths.Clamp01(t);
            return t * t * (3f - 2f * t);
        }

        /// <summary>Starts slow.</summary>
        [LocalSafe]
        public static float InQuad(float t)
        {
            t = Maths.Clamp01(t);
            return t * t;
        }

        /// <summary>Ends slow.</summary>
        [LocalSafe]
        public static float OutQuad(float t)
        {
            t = Maths.Clamp01(t);
            return 1f - (1f - t) * (1f - t);
        }

        /// <summary>Slow at both ends.</summary>
        [LocalSafe]
        public static float InOutQuad(float t)
        {
            t = Maths.Clamp01(t);
            if (t < 0.5f)
            {
                return 2f * t * t;
            }

            var u = -2f * t + 2f;
            return 1f - u * u / 2f;
        }

        /// <summary>Starts slower than InQuad.</summary>
        [LocalSafe]
        public static float InCubic(float t)
        {
            t = Maths.Clamp01(t);
            return t * t * t;
        }

        /// <summary>Ends slower than OutQuad.</summary>
        [LocalSafe]
        public static float OutCubic(float t)
        {
            t = Maths.Clamp01(t);
            var u = 1f - t;
            return 1f - u * u * u;
        }

        /// <summary>Slow at both ends, more than InOutQuad.</summary>
        [LocalSafe]
        public static float InOutCubic(float t)
        {
            t = Maths.Clamp01(t);
            if (t < 0.5f)
            {
                return 4f * t * t * t;
            }

            var u = -2f * t + 2f;
            return 1f - u * u * u / 2f;
        }

        /// <summary>Starts slow, along a sine.</summary>
        [LocalSafe]
        public static float InSine(float t)
        {
            t = Maths.Clamp01(t);
            return 1f - Trig.Cos(t * HalfPi);
        }

        /// <summary>Ends slow, along a sine.</summary>
        [LocalSafe]
        public static float OutSine(float t)
        {
            t = Maths.Clamp01(t);
            return Trig.Sin(t * HalfPi);
        }

        /// <summary>Slow at both ends, along a sine.</summary>
        [LocalSafe]
        public static float InOutSine(float t)
        {
            t = Maths.Clamp01(t);
            return (1f - Trig.Cos(t * MathF.PI)) / 2f;
        }

        /// <summary>Pulls back below 0 first, then goes.</summary>
        [LocalSafe]
        public static float InBack(float t)
        {
            t = Maths.Clamp01(t);
            return (Back + 1f) * t * t * t - Back * t * t;
        }

        /// <summary>Overshoots past 1, then settles.</summary>
        [LocalSafe]
        public static float OutBack(float t)
        {
            t = Maths.Clamp01(t);
            var u = t - 1f;
            return 1f + (Back + 1f) * u * u * u + Back * u * u;
        }

        /// <summary>Pulls back at the start and overshoots at the end.</summary>
        [LocalSafe]
        public static float InOutBack(float t)
        {
            t = Maths.Clamp01(t);
            if (t < 0.5f)
            {
                var a = 2f * t;
                return a * a * ((BackInOut + 1f) * a - BackInOut) / 2f;
            }

            var b = 2f * t - 2f;
            return (b * b * ((BackInOut + 1f) * b + BackInOut) + 2f) / 2f;
        }

        /// <summary>Bounces at the start.</summary>
        [LocalSafe]
        public static float InBounce(float t)
        {
            return 1f - OutBounce(1f - Maths.Clamp01(t));
        }

        /// <summary>Bounces at the end, as a dropped ball.</summary>
        [LocalSafe]
        public static float OutBounce(float t)
        {
            t = Maths.Clamp01(t);
            const float n = 7.5625f;
            const float d = 2.75f;
            if (t < 1f / d)
            {
                return n * t * t;
            }

            if (t < 2f / d)
            {
                t -= 1.5f / d;
                return n * t * t + 0.75f;
            }

            if (t < 2.5f / d)
            {
                t -= 2.25f / d;
                return n * t * t + 0.9375f;
            }

            t -= 2.625f / d;
            return n * t * t + 0.984375f;
        }

        /// <summary>Bounces at both ends.</summary>
        [LocalSafe]
        public static float InOutBounce(float t)
        {
            t = Maths.Clamp01(t);
            if (t < 0.5f)
            {
                return (1f - OutBounce(1f - 2f * t)) / 2f;
            }

            return (1f + OutBounce(2f * t - 1f)) / 2f;
        }
    }
}
