using System.Text;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases from the full audit's ground ray finding MT1: a bounce or a roll from a hit's point goes on, as a map's bounce
    /// loop moves its object to the hit's point. The grids are 10 by 10 corners from (0, 0), 1,152 across.
    /// </summary>
    public static class GroundBounceCases
    {
        /// <summary>Installs a grid of 10 by 10 corners from (0, 0) whose corner (i, j) is at height rise times i.</summary>
        private static void Slope(float rise)
        {
            var text = new StringBuilder();
            for (var j = 0; j < 10; j++)
            {
                for (var i = 0; i < 10; i++)
                {
                    text.Append(GridText.Corner(rise * i, 2));
                }
            }

            Terrain.Install(new FakeTerrainWorld(), 10, 10, 0f, 0f, text.ToString());
        }

        /// <summary>Moves a point a step at a time against the ground for some steps, bouncing or sliding off it where a step hits; gives the hits, and counts the steps that did not move it in stuck.</summary>
        private static int Run(ref Vector3 position, ref Vector3 velocity, int steps, bool slides, out int stuck)
        {
            var hits = 0;
            stuck = 0;
            for (var i = 0; i < steps; i++)
            {
                var before = position;
                if (Terrain.Raycast(position, position + velocity, out var hit))
                {
                    hits++;
                    position = hit.Point;
                    velocity = slides ? velocity.Slide(hit.Normal) : velocity.Bounce(hit.Normal);
                }
                else
                {
                    position = position + velocity;
                }

                if (position == before)
                {
                    stuck++;
                }
            }

            return hits;
        }

        /// <summary>A point that falls onto flat ground bounces off it once and goes up and on (it used to hit again at 0 and stay).</summary>
        public static void BounceFlat(FakeWorld world)
        {
            Slope(0f);
            var position = new Vector3(200f, 300f, 40f);
            var velocity = new Vector3(8f, 4f, -16f);
            var hits = Run(ref position, ref velocity, 20, false, out var stuck);
            Expect.Equal(1, hits, "hits");
            Expect.Equal(0, stuck, "steps that did not move");
            // Under the ground halfway through the third step, at (220, 310, 0); then up at 16 for 17 steps.
            Expect.Near(272f, position.Z, "went up from the ground");
            Expect.Near(356f, position.X, "went on along x");
        }

        /// <summary>A point that falls onto a slope of 1 in 2 bounces off it once, up and back down the slope, and goes on.</summary>
        public static void BounceSlope(FakeWorld world)
        {
            Slope(64f);
            // The ground under (600, 500) is at 300 exactly, so the hit's point is on it exactly.
            var position = new Vector3(600f, 500f, 400f);
            var velocity = new Vector3(0f, 0f, -10f);
            var hits = Run(ref position, ref velocity, 30, false, out var stuck);
            Expect.Equal(1, hits, "hits");
            Expect.Equal(0, stuck, "steps that did not move");
            // Off the normal (-1, 0, 2) / sqrt(5), (0, 0, -10) bounces to (-8, 0, 6).
            Expect.True(velocity.X < -7.9f && velocity.X > -8.1f && velocity.Z > 5.9f && velocity.Z < 6.1f, "bounced to (-8, 0, 6), was " + velocity);
            var gap = position.Z - Terrain.Height(position.X, position.Y);
            Expect.True(gap > 150f, "went up from the slope, " + gap + " above it");
        }

        /// <summary>A point on flat ground that moves along it rolls on, and one that slides after a hit slides on (they used to hit at 0 each step).</summary>
        public static void RollFlat(FakeWorld world)
        {
            Slope(0f);
            var position = new Vector3(200f, 300f, 0f);
            var velocity = new Vector3(8f, 4f, 0f);
            var hits = Run(ref position, ref velocity, 20, true, out var stuck);
            Expect.Equal(0, hits, "rolling: hits");
            Expect.Equal(0, stuck, "rolling: steps that did not move");
            Expect.Near(360f, position.X, "rolled along x");
            position = new Vector3(200f, 300f, 20f);
            velocity = new Vector3(8f, 4f, -16f);
            hits = Run(ref position, ref velocity, 20, true, out stuck);
            Expect.Equal(1, hits, "sliding: hits");
            Expect.Equal(0, stuck, "sliding: steps that did not move");
            // Under the ground a quarter through the second step, at (210, 305, 0); then along x at 8 for 18 steps.
            Expect.Near(354f, position.X, "slid on along x");
            Expect.True(position.Z > -0.02f && position.Z < 0.02f, "slid on the ground, at " + position.Z);
        }

        /// <summary>A point on a slope that moves along it, up and across, rolls on.</summary>
        public static void RollSlope(FakeWorld world)
        {
            Slope(64f);
            var x = 300.3f;
            var y = 200.7f;
            var position = new Vector3(x, y, Terrain.Height(x, y));
            var velocity = new Vector3(8f, 3f, 4f);
            var hits = Run(ref position, ref velocity, 30, true, out var stuck);
            Expect.Equal(0, hits, "hits");
            Expect.Equal(0, stuck, "steps that did not move");
            Expect.Near(x + 240f, position.X, "rolled up the slope");
        }

        /// <summary>A point on the ground that moves into it still hits it at once (where it is 0.01 under), on the ground, with the ground's normal.</summary>
        public static void IntoFromGround(FakeWorld world)
        {
            Slope(64f);
            var start = new Vector3(300f, 200f, 150f);
            Expect.True(Terrain.Raycast(start, start + new Vector3(8f, 0f, -10f), out var hit), "hits");
            Expect.True(hit.Fraction < 0.01f, "at once: fraction " + hit.Fraction);
            Expect.True(hit.Point.DistanceTo(start) < 0.02f, "beside its start, " + hit.Point.DistanceTo(start) + " from it");
            Expect.Near(Terrain.Height(hit.Point.X, hit.Point.Y), hit.Point.Z, "on the ground");
            Expect.True(hit.Normal == Terrain.Normal(hit.Point.X, hit.Point.Y), "the ground's normal");
            Expect.True(Terrain.Raycast(new Vector3(300f, 200f, 140f), new Vector3(308f, 200f, 200f), out hit), "under the ground: hits");
            Expect.True(hit.Normal == Vector3.UnitZ, "under the ground: straight up, as before");
        }
    }
}
