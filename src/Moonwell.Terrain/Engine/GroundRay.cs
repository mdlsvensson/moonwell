using System;

namespace Moonwell.Engine
{
    /// <summary>
    /// A segment against the blended ground. The segment is sampled every 32 along its length in XY (and at its end);
    /// at the first sample at or under the ground the span back to the sample before is bisected five times, then the
    /// last span is crossed by a straight line between its two gaps, so a ray straight down (one sample, at its end)
    /// still lands on the ground. All of it is worked out in floats: a miss makes nothing.
    /// </summary>
    internal static class GroundRay
    {
        /// <summary>The distance between samples along the segment's length in XY.</summary>
        private const float Step = 32f;

        /// <summary>How many times the span of the first sample under the ground is halved.</summary>
        private const int Bisections = 5;

        /// <summary>The empty hit every miss gives, made on first use and shared (a TerrainHit never changes).</summary>
        private static TerrainHit[]? none;

        /// <summary>Casts the segment from (fromX, fromY, fromZ) to (toX, toY, toZ) against the ground. Gives the hit, or an empty hit on a miss.</summary>
        internal static bool Cast(float fromX, float fromY, float fromZ, float toX, float toY, float toZ, out TerrainHit hit)
        {
            var dx = toX - fromX;
            var dy = toY - fromY;
            var dz = toZ - fromZ;
            var startGap = fromZ - Terrain.Height(fromX, fromY);
            if (startGap <= 0f)
            {
                hit = new TerrainHit(new Vector3(fromX, fromY, fromZ), Vector3.UnitZ, 0f);
                return true;
            }

            var length = MathF.Sqrt(dx * dx + dy * dy);
            var before = 0f;
            var gapBefore = startGap;
            var k = 0;
            while (true)
            {
                k++;
                var t = length > 0f ? k * Step / length : 1f;
                if (t > 1f)
                {
                    t = 1f;
                }

                var gap = Gap(fromX, fromY, fromZ, dx, dy, dz, t);
                if (gap <= 0f)
                {
                    hit = HitBetween(fromX, fromY, fromZ, dx, dy, dz, before, gapBefore, t, gap);
                    return true;
                }

                if (t >= 1f)
                {
                    hit = Miss();
                    return false;
                }

                before = t;
                gapBefore = gap;
            }
        }

        /// <summary>The hit in the span from a fraction above the ground to one at or under it.</summary>
        private static TerrainHit HitBetween(float fromX, float fromY, float fromZ, float dx, float dy, float dz, float above, float gapAbove, float under, float gapUnder)
        {
            for (var i = 0; i < Bisections; i++)
            {
                var middle = (above + under) / 2f;
                var gap = Gap(fromX, fromY, fromZ, dx, dy, dz, middle);
                if (gap > 0f)
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

            var fraction = above + (under - above) * gapAbove / (gapAbove - gapUnder);
            var x = fromX + dx * fraction;
            var y = fromY + dy * fraction;
            return new TerrainHit(new Vector3(x, y, fromZ + dz * fraction), Terrain.Normal(x, y), fraction);
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
