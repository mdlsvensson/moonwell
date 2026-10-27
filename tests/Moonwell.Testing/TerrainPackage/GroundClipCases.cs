namespace Moonwell.Testing
{
    /// <summary>
    /// Cases from the full audit's ground ray finding MT4: only the part of a segment over the map's grid is cast, so a long
    /// one costs no more than one across the map, and one past the edge does not hit out there. The grids are one tile.
    /// </summary>
    public static class GroundClipCases
    {
        /// <summary>Installs flat ground at 0, one tile from (0, 0) to (128, 128).</summary>
        private static void OneTile()
        {
            Terrain.Install(new FakeTerrainWorld(), 2, 2, 0f, 0f, GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
        }

        /// <summary>A ray that leaves the grid and would go under the edge's height out there does not hit past the edge (it used to, at x 4064).</summary>
        public static void PastTheEdge(FakeWorld world)
        {
            OneTile();
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 100f), new Vector3(8064f, 64f, -100f), out _), "past the edge: no hit");
            Expect.True(!Terrain.Raycast(new Vector3(-1000f, -1000f, -50f), new Vector3(-900f, -1000f, -50f), out _), "wholly outside the grid, under the edge's height: no hit");
            // Down 1 for 1 from 20 above the edge: it reaches the ground at x 20, 1,020 along a segment of 2,000.
            Expect.True(Terrain.Raycast(new Vector3(-1000f, 64f, 1020f), new Vector3(1000f, 64f, -980f), out var hit), "coming in over the edge: hits");
            Vector2Cases.Near(new Vector2(20f, 64f), new Vector2(hit.Point.X, hit.Point.Y), "inside the edge");
            Expect.Near(0f, hit.Point.Z, "on the ground");
            Expect.Near(0.51f, hit.Fraction, "fraction along the whole segment");
        }

        /// <summary>A ray a million long reads the ground only over the grid: at a few points, not 31,250.</summary>
        public static void LongRayClipped(FakeWorld world)
        {
            OneTile();
            var before = Engine.GroundRay.Samples;
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 50f), new Vector3(1000000f, 64f, 50f), out _), "no hit");
            var samples = Engine.GroundRay.Samples - before;
            Expect.True(samples <= 4, "read the ground " + samples + " times");
            before = Engine.GroundRay.Samples;
            Expect.True(Terrain.Raycast(new Vector3(64f, 64f, 50f), new Vector3(-1000000f, 64f, -1000000f), out var hit), "a long one down: hits");
            Expect.Near(14f, hit.Point.X, "where it meets the ground, 50 on from its start");
            samples = Engine.GroundRay.Samples - before;
            Expect.True(samples <= 4, "a long one down read the ground " + samples + " times");
        }
    }
}
