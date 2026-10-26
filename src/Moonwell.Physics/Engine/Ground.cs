namespace Moonwell.Engine
{
    /// <summary>
    /// The ground as the package sees it: the terrain's grid (water's surface where it is above the ground), or a placed
    /// bridge's deck where one stands. Every height the package reads goes through it, and every read is in floats: a read
    /// makes no table. A read of the height costs as much as the terrain's (9.4 µs in the game), so the pass reads it
    /// only when a missile may have reached the ground.
    /// </summary>
    internal static class Ground
    {
        /// <summary>The distance between samples along a cast's length in XY, as the terrain's own raycast.</summary>
        private const float Step = 32.0f;

        /// <summary>How many times a cast halves the span of its first sample under the ground.</summary>
        private const int Bisections = 5;

        /// <summary>How many times the height has been read; the cases read it to see how often the ground is read.</summary>
        internal static int reads;

        /// <summary>The ground's height at a point for a ground unit: the deck where one stands, else the terrain.</summary>
        internal static float Height(float x, float y)
        {
            reads++;
            if (Decks.count > 0)
            {
                float deck;
                if (Decks.Under(x, y, out deck))
                {
                    return deck;
                }
            }

            return Terrain.Height(x, y);
        }

        /// <summary>The ground's height at a point for something at a height: the highest of the terrain and any deck there that is not above that height, so a deck is met from above only.</summary>
        internal static float Height(float x, float y, float z)
        {
            reads++;
            var height = Terrain.Height(x, y);
            if (Decks.count > 0)
            {
                float deck;
                if (Decks.Top(x, y, z, out deck) && deck > height)
                {
                    return deck;
                }
            }

            return height;
        }

        /// <summary>The direction straight out of the ground at a point, for something at a height, of length 1. In a cliff tile it is made level, so a cliff's face is met as a wall, not up its blend.</summary>
        internal static void Normal(float x, float y, float z, out float nx, out float ny, out float nz)
        {
            if (Decks.count > 0 && Decks.Normal(x, y, z, out nx, out ny, out nz))
            {
                return;
            }

            var grid = Terrain.Grid;
            float alongX;
            float alongY;
            grid.Gradient(x, y, out alongX, out alongY);
            if (grid.IsCliff(x, y))
            {
                var level = Trig.Sqrt(alongX * alongX + alongY * alongY);
                if (level > 0.0f)
                {
                    nx = -alongX / level;
                    ny = -alongY / level;
                    nz = 0.0f;
                    return;
                }
            }

            Out(alongX, alongY, out nx, out ny, out nz);
        }

        /// <summary>The direction straight out of the ground's blend at a point, for something at a height, of length 1: as Normal, but not made level in a cliff tile. Rolling follows it.</summary>
        internal static void Slope(float x, float y, float z, out float nx, out float ny, out float nz)
        {
            if (Decks.count > 0 && Decks.Normal(x, y, z, out nx, out ny, out nz))
            {
                return;
            }

            float alongX;
            float alongY;
            Terrain.Grid.Gradient(x, y, out alongX, out alongY);
            Out(alongX, alongY, out nx, out ny, out nz);
        }

        /// <summary>
        /// The first place the segment from (x0, y0, z0) to (x1, y1, z1) meets the ground, as the terrain's raycast does,
        /// with the decks not above the segment's start: sampled every 32 along its length in XY and at its end, the span
        /// of the first sample at or under the ground halved five times, then crossed by a straight line. A segment that
        /// starts under the ground hits at fraction 0, on the ground above its start. One that stays above it, or has a NaN
        /// or an infinite end, gives false. Gives the point, the fraction along the segment, and how far the start is above
        /// the ground (0 when it was not read).
        /// </summary>
        internal static bool Cast(float x0, float y0, float z0, float x1, float y1, float z1, out float hitX, out float hitY, out float hitZ, out float fraction, out float startGap)
        {
            hitX = 0.0f;
            hitY = 0.0f;
            hitZ = 0.0f;
            fraction = 0.0f;
            startGap = 0.0f;
            var dx = x1 - x0;
            var dy = y1 - y0;
            var dz = z1 - z0;
            if (!Finite(dx) || !Finite(dy) || !Finite(dz) || !Finite(x0) || !Finite(y0) || !Finite(z0))
            {
                return false;
            }

            startGap = z0 - Height(x0, y0, z0);
            if (startGap < 0.0f)
            {
                hitX = x0;
                hitY = y0;
                hitZ = z0 - startGap;
                return true;
            }

            var length = Trig.Sqrt(dx * dx + dy * dy);
            var above = 0.0f;
            var gapAbove = startGap;
            var k = 0;
            while (true)
            {
                k++;
                var t = length > 0.0f ? k * Step / length : 1.0f;
                if (!(t < 1.0f))
                {
                    t = 1.0f;
                }

                var gap = Gap(x0, y0, z0, dx, dy, dz, t);
                if (gap <= 0.0f)
                {
                    var under = t;
                    var gapUnder = gap;
                    for (var i = 0; i < Bisections; i++)
                    {
                        var middle = (above + under) * 0.5f;
                        var midGap = Gap(x0, y0, z0, dx, dy, dz, middle);
                        if (midGap > 0.0f)
                        {
                            above = middle;
                            gapAbove = midGap;
                        }
                        else
                        {
                            under = middle;
                            gapUnder = midGap;
                        }
                    }

                    // A segment that starts on the ground has a gap of 0 there: then the span may have no gap to cross.
                    var span = gapAbove - gapUnder;
                    var at = span > 0.0f ? above + (under - above) * gapAbove / span : above;
                    fraction = at;
                    hitX = x0 + dx * at;
                    hitY = y0 + dy * at;
                    hitZ = z0 + dz * at;
                    return true;
                }

                if (t >= 1.0f)
                {
                    return false;
                }

                above = t;
                gapAbove = gap;
            }
        }

        /// <summary>The unit normal of ground whose height changes by alongX along x and alongY along y.</summary>
        private static void Out(float alongX, float alongY, out float nx, out float ny, out float nz)
        {
            var length = Trig.Sqrt(alongX * alongX + alongY * alongY + 1.0f);
            nx = -alongX / length;
            ny = -alongY / length;
            nz = 1.0f / length;
        }

        /// <summary>How far a segment is above the ground at a fraction of its length, with the decks not above its start.</summary>
        private static float Gap(float x0, float y0, float z0, float dx, float dy, float dz, float t)
        {
            return z0 + dz * t - Height(x0 + dx * t, y0 + dy * t, z0);
        }

        /// <summary>Whether a number is neither NaN nor infinite: only then is it taken away from itself to 0.</summary>
        private static bool Finite(float value)
        {
            return value - value == 0.0f;
        }
    }
}
