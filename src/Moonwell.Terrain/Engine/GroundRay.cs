using System;

namespace Moonwell.Engine
{
    /// <summary>
    /// A segment against the blended ground. The segment is sampled every 32 along its length in XY (and at its end);
    /// at the first sample at or under the ground the span back to the sample before is bisected five times, then the
    /// last span is crossed by a straight line between its two gaps, so a ray straight down (one sample, at its end)
    /// still lands on the ground. As Raycast's shapes, a segment that starts on the ground (within 0.02 of it) hits only
    /// where it goes under by more than 0.01 and deeper than it started, and the hit is put on the ground there, so a
    /// bounce or a roll from a hit's point goes on. A segment with a NaN or an infinite end misses at once (it would never
    /// reach its end). All of it is worked out in floats: a miss makes nothing.
    /// </summary>
    internal static class GroundRay
    {
        /// <summary>The distance between samples along the segment's length in XY.</summary>
        private const float Step = 32f;

        /// <summary>How many times the span of the first sample under the ground is halved.</summary>
        private const int Bisections = 5;

        /// <summary>How far under the ground a segment that starts on it must go to hit it, as Raycast's skin: above the blend's and a float's rounding of a hit's point, and below anything the game shows.</summary>
        private const float Skin = 0.01f;

        /// <summary>How close to the ground a start counts as on it, as Raycast's surface: twice the skin, so a start that rounding has carried a little under the skin is still on the ground, and the next hit puts it back on it.</summary>
        private const float Surface = 0.02f;

        /// <summary>The empty hit every miss gives, made on first use and shared (a TerrainHit never changes).</summary>
        private static TerrainHit[]? none;

        /// <summary>Casts the segment from (fromX, fromY, fromZ) to (toX, toY, toZ) against the ground. Gives the hit, or an empty hit on a miss.</summary>
        internal static bool Cast(float fromX, float fromY, float fromZ, float toX, float toY, float toZ, out TerrainHit hit)
        {
            var dx = toX - fromX;
            var dy = toY - fromY;
            var dz = toZ - fromZ;
            if (!Finite(dx) || !Finite(dy) || !Finite(dz) || !Finite(fromX) || !Finite(fromY) || !Finite(fromZ))
            {
                hit = Miss();
                return false;
            }

            var startGap = fromZ - Terrain.Height(fromX, fromY);
            if (startGap < -Surface)
            {
                hit = new TerrainHit(new Vector3(fromX, fromY, fromZ), Vector3.UnitZ, 0f);
                return true;
            }

            // A segment that starts on the ground hits only where it goes under it by more than the skin and deeper than it
            // started, until it has risen clear of it.
            var floor = startGap > Surface ? 0f : (startGap < -Skin ? startGap : -Skin);

            var length = MathF.Sqrt(dx * dx + dy * dy);
            if (!Finite(length))
            {
                hit = Miss();
                return false;
            }

            var before = 0f;
            var gapBefore = startGap;
            var k = 0;
            while (true)
            {
                k++;
                var t = length > 0f ? k * Step / length : 1f;
                if (!(t < 1f))
                {
                    t = 1f;
                }

                var gap = Gap(fromX, fromY, fromZ, dx, dy, dz, t);
                if (gap <= floor)
                {
                    hit = HitBetween(fromX, fromY, fromZ, dx, dy, dz, before, gapBefore, t, gap, floor);
                    return true;
                }

                if (t >= 1f)
                {
                    hit = Miss();
                    return false;
                }

                if (gap > Skin)
                {
                    floor = 0f;
                }

                before = t;
                gapBefore = gap;
            }
        }

        /// <summary>Whether a number is neither NaN nor infinite: only then is it taken away from itself to 0.</summary>
        private static bool Finite(float value)
        {
            return value - value == 0f;
        }

        /// <summary>
        /// The hit in the span from a fraction above the floor to one at or under it: where the segment reaches the floor,
        /// the span halved five times, then crossed by a straight line. The floor is the ground for a segment that came from
        /// above it. For one that started on the ground it is the skin under the ground (or the segment's start, if that is
        /// deeper), and the hit is put on the ground there: a map that moves its missile to the hit's point keeps it on the
        /// ground however a float rounds its steps, and a roll through a hollow goes on.
        /// </summary>
        private static TerrainHit HitBetween(float fromX, float fromY, float fromZ, float dx, float dy, float dz, float above, float gapAbove, float under, float gapUnder, float floor)
        {
            for (var i = 0; i < Bisections; i++)
            {
                var middle = (above + under) / 2f;
                var gap = Gap(fromX, fromY, fromZ, dx, dy, dz, middle);
                if (gap > floor)
                {
                    above = middle;
                    gapAbove = gap;
                }
                else
                {
                    under = middle;
                    gapUnder = gap;
                }
            }

            var fraction = gapAbove > floor ? above + (under - above) * (gapAbove - floor) / (gapAbove - gapUnder) : above;
            var x = fromX + dx * fraction;
            var y = fromY + dy * fraction;
            var z = floor < 0f ? Terrain.Height(x, y) : fromZ + dz * fraction;
            return new TerrainHit(new Vector3(x, y, z), Terrain.Normal(x, y), fraction);
        }

        /// <summary>How far the segment is above the ground at a fraction of its length.</summary>
        private static float Gap(float fromX, float fromY, float fromZ, float dx, float dy, float dz, float t)
        {
            return fromZ + dz * t - Terrain.Height(fromX + dx * t, fromY + dy * t);
        }

        /// <summary>The empty hit of a miss: one shared value, so a miss makes nothing.</summary>
        private static TerrainHit Miss()
        {
            if (none == null)
            {
                none = new[] { new TerrainHit(Vector3.Zero, Vector3.Zero, 0f) };
            }

            return none[0];
        }
    }
}
