namespace Moonwell.Testing
{
    /// <summary>
    /// A raycast that misses makes nothing in Lua, as Raycast's summary says, and Ray and RayHit compare and print as the
    /// other values do. The memory is read in the Lua host, where the collector can be stopped; on .NET the reads give 0.
    /// </summary>
    public static class MissCases
    {
        /// <summary>A thousand misses of each raycast leave Lua's memory where it was.</summary>
        public static void MissMakesNothing(FakeWorld world)
        {
            var from = new Vector2(-10f, 30f);
            var to = new Vector2(10f, 30f);
            var center = Vector2.Zero;
            var a = new Vector2(0f, -5f);
            var b = new Vector2(0f, 5f);
            var box = new Bounds(Vector2.Zero, new Vector2(10f, 10f));
            var square = new Polygon(new Vector2(0f, 0f), new Vector2(10f, 0f), new Vector2(10f, 10f), new Vector2(0f, 10f));
            var near = new Vector2(-10f, 5f);
            var short_ = new Vector2(-5f, 5f);
            var hits = 0;
            // A first round, so anything made once (a shared empty hit) is made before the count.
            for (var i = 0; i < 2; i++)
            {
                hits += Misses(from, to, near, short_, center, a, b, box, square);
            }

            StopCollector();
            var before = MemoryKb();
            for (var i = 0; i < 1000; i++)
            {
                hits += Misses(from, to, near, short_, center, a, b, box, square);
            }

            var grown = MemoryKb() - before;
            RestartCollector();
            Expect.Equal(0, hits, "every raycast missed");
            Expect.True(grown < 1f, "a thousand rounds of misses grew Lua's memory by " + grown + " KB");
        }

        private static int Misses(Vector2 from, Vector2 to, Vector2 near, Vector2 short_, Vector2 center, Vector2 a, Vector2 b, Bounds box, Polygon square)
        {
            var hits = 0;
            if (Raycast.Circle(from, to, center, 2f, out _))
            {
                hits++;
            }

            if (Raycast.Segment(from, to, a, b, out _))
            {
                hits++;
            }

            if (Raycast.Bounds(from, to, box, out _))
            {
                hits++;
            }

            if (Raycast.Polygon(from, to, square, out _))
            {
                hits++;
            }

            // Overlapping the polygon's bounds' row but ending short of it.
            if (Raycast.Polygon(near, short_, square, out _))
            {
                hits++;
            }

            return hits;
        }

        /// <summary>Ray and RayHit have ==, Equals, a hash and text, as Vector2.</summary>
        public static void RayValues(FakeWorld world)
        {
            var ray = new Ray(Vector2.Zero, new Vector2(0f, 5f));
            Expect.True(ray == new Ray(Vector2.Zero, Vector2.Up), "ray ==");
            Expect.True(ray != new Ray(Vector2.One, Vector2.Up), "ray !=");
            Expect.True(ray.Equals((object)new Ray(Vector2.Zero, Vector2.Up)), "ray Equals(object)");
            Expect.Equal(ray.GetHashCode(), new Ray(Vector2.Zero, Vector2.Up).GetHashCode(), "ray hash");
            Expect.Equal("((0, 0), (0, 1))", ray.ToString(), "ray text");
            var hit = new RayHit(new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f);
            Expect.True(hit == new RayHit(new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f), "hit ==");
            Expect.True(hit != new RayHit(new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.25f), "hit !=");
            Expect.True(hit.Equals(new RayHit(new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f)), "hit Equals");
            Expect.True(!hit.Equals("x"), "hit not Equals(\"x\")");
            Expect.Equal(hit.GetHashCode(), new RayHit(new Vector2(0f, 5f), new Vector2(-1f, 0f), 0.5f).GetHashCode(), "hit hash");
            Expect.Equal("((0, 5), (-1, 0), 0.5)", hit.ToString(), "hit text");
        }

        // Lua's memory in KB; the game's Lua has no collectgarbage, but these cases run in the Lua host and on .NET only.
        /// @CSharpLua.Template = "collectgarbage('count')"
        private static float MemoryKb()
        {
            return 0f;
        }

        /// @CSharpLua.Template = "collectgarbage('stop')"
        private static void StopCollector()
        {
        }

        /// @CSharpLua.Template = "collectgarbage('restart')"
        private static void RestartCollector()
        {
        }
    }
}
