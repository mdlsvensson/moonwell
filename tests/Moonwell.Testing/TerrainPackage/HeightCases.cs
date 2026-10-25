using System;

namespace Moonwell.Testing
{
    /// <summary>Cases for Terrain.Height, IsCliff, Normal and Slope, on small grids made with GridText: one tile from (0, 0) to (128, 128) unless a case says otherwise.</summary>
    public static class HeightCases
    {
        /// <summary>Installs a one-tile grid at (0, 0) on a fake world.</summary>
        private static void OneTile(string text)
        {
            Terrain.Install(new FakeTerrainWorld(), 2, 2, 0f, 0f, text);
        }

        /// <summary>Height at each corner is the corner's z.</summary>
        public static void Corners(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(10f, 2), GridText.Corner(20f, 2), GridText.Corner(30f, 2), GridText.Corner(40f, 2)));
            Expect.Near(10f, Terrain.Height(0f, 0f), "bottom left");
            Expect.Near(20f, Terrain.Height(128f, 0f), "bottom right");
            Expect.Near(30f, Terrain.Height(0f, 128f), "top left");
            Expect.Near(40f, Terrain.Height(128f, 128f), "top right");
        }

        /// <summary>Between the corners of one level the height is bilinear.</summary>
        public static void Blend(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 2), GridText.Corner(0f, 2), GridText.Corner(128f, 2)));
            Expect.Near(64f, Terrain.Height(64f, 0f), "halfway along the bottom edge");
            Expect.Near(64f, Terrain.Height(64f, 64f), "the middle");
            Expect.Near(32f, Terrain.Height(32f, 100f), "a quarter across");
            Expect.True(!Terrain.IsCliff(64f, 64f), "one level is no cliff");
        }

        /// <summary>A point outside the grid reads the edge corner nearest it.</summary>
        public static void Clamped(FakeWorld world)
        {
            Terrain.Install(new FakeTerrainWorld(), 2, 2, -256f, -256f, GridText.Tile(GridText.Corner(10f, 2), GridText.Corner(20f, 2), GridText.Corner(30f, 2), GridText.Corner(40f, 2)));
            Expect.Near(10f, Terrain.Height(-756f, -300f), "500 left of left, below the bottom");
            Expect.Near(30f, Terrain.Height(-756f, 0f), "500 left of left, above the top");
            Expect.Near(40f, Terrain.Height(1000f, 1000f), "past the top right corner");
            Expect.Near(20f, Terrain.Height(1000f, -1000f), "past the bottom right corner");
            Expect.Near(35f, Terrain.Height(-192f, 500f), "above the top edge keeps its place along it");
            Expect.True(!Terrain.IsCliff(-756f, 500f), "outside is no cliff on one level");
        }

        /// <summary>A tile with levels 2 and 3 on its left and right: a point takes the level of the nearer side.</summary>
        public static void Step(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(0f, 2), GridText.Corner(128f, 3)));
            Expect.Near(0f, Terrain.Height(32f, 64f), "fx 0.25 is the left corners'");
            Expect.Near(128f, Terrain.Height(96f, 64f), "fx 0.75 is the right corners'");
            Expect.True(Terrain.IsCliff(32f, 64f), "IsCliff on the left");
            Expect.True(Terrain.IsCliff(96f, 64f), "IsCliff on the right");
            var normal = Terrain.Normal(32f, 64f);
            Expect.Near(0f, normal.X, "normal x");
            Expect.Near(0f, normal.Y, "normal y");
            Expect.Near(1f, normal.Z, "normal z");
            Expect.Near(0f, Terrain.Slope(96f, 64f), "slope");
        }

        /// <summary>Within a level, only the corners on it blend, their weights divided by their sum.</summary>
        public static void StepBlend(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(64f, 2), GridText.Corner(128f, 3)));
            Expect.Near(16f, Terrain.Height(32f, 32f), "fx 0.25, fy 0.25: 64 x 0.1875 / 0.75");
            Expect.Near(48f, Terrain.Height(32f, 96f), "fx 0.25, fy 0.75");
            Expect.Near(128f, Terrain.Height(96f, 32f), "the right level has one z");
        }

        /// <summary>On the line halfway across a step tile, the point takes the higher corners', right and top.</summary>
        public static void StepTie(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(0f, 2), GridText.Corner(128f, 3)));
            Expect.Near(128f, Terrain.Height(64f, 20f), "fx exactly 0.5 takes the right level");
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(128f, 3)));
            Expect.Near(128f, Terrain.Height(20f, 64f), "fy exactly 0.5 takes the top level");
            Expect.Near(0f, Terrain.Height(20f, 63f), "just under 0.5 takes the bottom level");
        }

        /// <summary>Three corners on one level and one higher: the point nearest the high corner is on its level, the rest on the other.</summary>
        public static void StepL(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(128f, 3)));
            Expect.Near(128f, Terrain.Height(96f, 96f), "near the high corner");
            Expect.Near(0f, Terrain.Height(32f, 96f), "near the top left");
            Expect.Near(0f, Terrain.Height(96f, 32f), "near the bottom right");
            Expect.True(Terrain.IsCliff(32f, 96f), "an L tile is a cliff");
        }

        /// <summary>The same levels with a ramp flag are blended and are no cliff.</summary>
        public static void Ramp(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2, true, false), GridText.Corner(128f, 3, true, false), GridText.Corner(0f, 2, true, false), GridText.Corner(128f, 3, true, false)));
            Expect.Near(32f, Terrain.Height(32f, 64f), "blended at fx 0.25");
            Expect.Near(96f, Terrain.Height(96f, 64f), "blended at fx 0.75");
            Expect.True(!Terrain.IsCliff(32f, 64f), "a ramp is no cliff");
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(0f, 2), GridText.Corner(128f, 3, true, false)));
            Expect.Near(32f, Terrain.Height(32f, 64f), "one ramp corner is enough");
            Expect.True(!Terrain.IsCliff(32f, 64f), "one ramp corner is no cliff");
        }

        /// <summary>A plane rising 128 over a tile is 45 degrees, with the normal leaning back; flat ground is straight up.</summary>
        public static void NormalAndSlope(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 2), GridText.Corner(0f, 2), GridText.Corner(128f, 2)));
            Expect.Near(45f, Terrain.Slope(40f, 70f), "slope along x");
            var normal = Terrain.Normal(40f, 70f);
            Expect.Near(-0.7071f, normal.X, "normal x");
            Expect.Near(0f, normal.Y, "normal y");
            Expect.Near(0.7071f, normal.Z, "normal z");
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(128f, 2), GridText.Corner(128f, 2)));
            Expect.Near(45f, Terrain.Slope(40f, 70f), "slope along y");
            normal = Terrain.Normal(40f, 70f);
            Expect.Near(0f, normal.X, "y plane: normal x");
            Expect.Near(-0.7071f, normal.Y, "y plane: normal y");
            Expect.Near(0.7071f, normal.Z, "y plane: normal z");
            OneTile(GridText.Tile(GridText.Corner(50f, 2), GridText.Corner(50f, 2), GridText.Corner(50f, 2), GridText.Corner(50f, 2)));
            normal = Terrain.Normal(40f, 70f);
            Expect.Near(0f, normal.X, "flat: normal x");
            Expect.Near(0f, normal.Y, "flat: normal y");
            Expect.Near(1f, normal.Z, "flat: normal z");
            Expect.Near(0f, Terrain.Slope(40f, 70f), "flat: slope");
        }

        /// <summary>Before Install, the reads throw.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            Terrain.Uninstall();
            Expect.Throws<InvalidOperationException>(() => Terrain.Height(0f, 0f), "Height");
            Expect.Throws<InvalidOperationException>(() => Terrain.IsCliff(0f, 0f), "IsCliff");
            Expect.Throws<InvalidOperationException>(() => Terrain.Normal(0f, 0f), "Normal");
            Expect.Throws<InvalidOperationException>(() => Terrain.Slope(0f, 0f), "Slope");
        }
    }
}
