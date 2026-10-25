using System;
using System.Text;

namespace Moonwell.Testing
{
    /// <summary>Cases for the terrain package's pathing flags, CanStand and LocalHeight, on a flat grid of 10 by 10 corners (36 by 36 cells of 32).</summary>
    public static class PathingCases
    {
        /// <summary>Installs a flat grid of 10 by 10 corners from (left, bottom) on a fake world; the cells run 36 across and 36 up.</summary>
        private static void Flat(FakeTerrainWorld fake, float left, float bottom)
        {
            var text = new StringBuilder();
            for (var k = 0; k < 10 * 10; k++)
            {
                text.Append(GridText.Corner(0f, 2));
            }

            Terrain.Install(fake, 10, 10, left, bottom, text.ToString());
        }

        /// <summary>Cell (3, 2) is unwalkable water in the fake: its centre and a point near its corner read so, and its neighbours read walkable and dry.</summary>
        public static void Flags(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            fake.Block(80f, 80f, FakeTerrainWorld.NoFloat | FakeTerrainWorld.NoFly);
            fake.Block(144f, 80f, FakeTerrainWorld.NoFloat | FakeTerrainWorld.NoBuild);
            fake.Block(112f, 48f, FakeTerrainWorld.NoFloat);
            fake.Block(112f, 112f, FakeTerrainWorld.NoFloat);
            fake.Block(112f, 80f, FakeTerrainWorld.NoWalk);
            Flat(fake, 0f, 0f);
            Expect.True(!Terrain.IsWalkable(112f, 80f), "the centre is not walkable");
            Expect.True(Terrain.IsWater(112f, 80f), "the centre is water");
            Expect.True(!Terrain.IsWalkable(97f, 65f), "the corner + (1, 1) is not walkable");
            Expect.True(Terrain.IsWater(97f, 65f), "the corner + (1, 1) is water");
            Expect.True(Terrain.IsFlyable(112f, 80f) && Terrain.IsBuildable(112f, 80f), "the centre can be flown and built on");
            Expect.True(Terrain.IsWalkable(80f, 80f) && Terrain.IsWalkable(144f, 80f), "the left and right neighbours walk");
            Expect.True(Terrain.IsWalkable(112f, 48f) && Terrain.IsWalkable(112f, 112f), "the lower and upper neighbours walk");
            Expect.True(!Terrain.IsWater(80f, 80f) && !Terrain.IsWater(144f, 80f), "the left and right neighbours are dry");
            Expect.True(!Terrain.IsWater(112f, 48f) && !Terrain.IsWater(112f, 112f), "the lower and upper neighbours are dry");
            Expect.True(!Terrain.IsFlyable(80f, 80f) && Terrain.IsFlyable(144f, 80f), "only the left neighbour cannot be flown");
            Expect.True(!Terrain.IsBuildable(144f, 80f) && Terrain.IsBuildable(80f, 80f), "only the right neighbour cannot be built on");
        }

        /// <summary>A block is read once, 256 cells at four calls a cell; the next block over reads its own.</summary>
        public static void FilledOnce(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            Expect.Equal(0, fake.PathCalls, "Install reads no cell");
            Expect.True(Terrain.IsWalkable(10f, 10f), "a cell in the first block");
            Expect.Equal(1024, fake.PathCalls, "one block is 256 cells at four calls");
            Expect.True(Terrain.IsBuildable(500f, 500f), "the last cell of the first block");
            Expect.True(Terrain.IsFlyable(40f, 300f), "another cell of the first block");
            Expect.Equal(1024, fake.PathCalls, "the same block is not read again");
            Expect.True(Terrain.IsWalkable(520f, 10f), "a cell in the next block over");
            Expect.Equal(2048, fake.PathCalls, "the next block is read in full");
        }

        /// <summary>The last block of a 36 by 36 cell grid is 4 by 4 cells: only those are read.</summary>
        public static void PartialBlock(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            Expect.True(Terrain.IsWalkable(1100f, 1100f), "the last cell");
            Expect.Equal(64, fake.PathCalls, "4 by 4 cells at four calls");
        }

        /// <summary>Refresh drops the blocks the area touches; the next read of one reads it again, and the others stay.</summary>
        public static void Refresh(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            Expect.True(Terrain.IsWalkable(112f, 80f), "open in the first block");
            Expect.True(Terrain.IsWalkable(600f, 80f), "open in the next block");
            Expect.Equal(2048, fake.PathCalls, "two blocks read");
            fake.Block(112f, 80f, FakeTerrainWorld.NoWalk);
            fake.Block(600f, 80f, FakeTerrainWorld.NoWalk);
            Expect.True(Terrain.IsWalkable(112f, 80f), "the first block keeps what it read");
            Terrain.Refresh(new Bounds(new Vector2(100f, 70f), new Vector2(130f, 90f)));
            Expect.Equal(2048, fake.PathCalls, "Refresh reads nothing itself");
            Expect.True(!Terrain.IsWalkable(112f, 80f), "the first block reads the new flag");
            Expect.Equal(3072, fake.PathCalls, "the first block was read again");
            Expect.True(Terrain.IsWalkable(600f, 80f), "the next block was not dropped");
            Expect.Equal(3072, fake.PathCalls, "the next block was not read again");
        }

        /// <summary>An area over two blocks drops both, also when it starts far outside the grid; a block it does not touch stays.</summary>
        public static void RefreshSpan(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            Expect.True(Terrain.IsWalkable(10f, 10f), "the first block");
            Expect.True(Terrain.IsWalkable(520f, 10f), "the second block");
            Expect.True(Terrain.IsWalkable(10f, 520f), "the block above the first");
            Expect.Equal(3072, fake.PathCalls, "three blocks read");
            Terrain.Refresh(new Bounds(new Vector2(-5000f, -5000f), new Vector2(520f, 10f)));
            Expect.True(Terrain.IsWalkable(10f, 10f) && Terrain.IsWalkable(520f, 10f), "both blocks again");
            Expect.Equal(5120, fake.PathCalls, "two blocks read again");
            Expect.True(Terrain.IsWalkable(10f, 520f), "the block above is untouched");
            Expect.Equal(5120, fake.PathCalls, "the block above was not read again");
        }

        /// <summary>A point outside the grid reads the edge cell, also where the grid starts at a negative position.</summary>
        public static void Outside(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            fake.Block(-240f, 80f, FakeTerrainWorld.NoWalk);
            fake.Block(-240f, -240f, FakeTerrainWorld.NoFly);
            fake.Block(-256f + 35f * 32f + 16f, -256f + 35f * 32f + 16f, FakeTerrainWorld.NoBuild);
            Flat(fake, -256f, -256f);
            Expect.True(!Terrain.IsWalkable(-1000f, 80f), "left of the grid reads the first cell of the row");
            Expect.True(!Terrain.IsFlyable(-1000f, -1000f), "below and left of the grid reads the first cell");
            Expect.True(!Terrain.IsBuildable(100000f, 100000f), "past the far corner reads the last cell");
            Expect.True(Terrain.IsWalkable(-250f, 200f), "inside the first cell column, another row");
            Expect.True(!Terrain.IsWalkable(-255f, 80f), "just inside the left edge");
            Expect.True(Terrain.IsWalkable(-257f, 200f), "just left of the edge reads the cell of that row");
        }

        /// <summary>A coordinate that is not a number reads the first cell of its row or column, and does not fail.</summary>
        public static void NotANumber(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            fake.Block(16f, 16f, FakeTerrainWorld.NoWalk | FakeTerrainWorld.NoFly | FakeTerrainWorld.NoBuild | FakeTerrainWorld.NoFloat);
            Flat(fake, 0f, 0f);
            Expect.True(!Terrain.IsWalkable(float.NaN, float.NaN), "NaN both ways reads the first cell: not walkable");
            Expect.True(!Terrain.IsFlyable(float.NaN, 0f), "NaN across reads the first cell of the row: not flyable");
            Expect.True(!Terrain.IsBuildable(0f, float.NaN), "NaN up reads the first cell of the column: not buildable");
            Expect.True(!Terrain.IsWater(float.NaN, float.NaN), "the first cell cannot float ships, and the read did not fail");
        }

        /// <summary>The item landing within 1 of the point is true; further is false, with where it landed.</summary>
        public static void CanStand(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            fake.Lands = true;
            fake.LandsX = 10f;
            fake.LandsY = 0f;
            Expect.True(Terrain.CanStand(10f, 0f, out var landed), "it lands where it was put");
            Expect.True(landed == new Vector2(10f, 0f), "nearest is the point");
            Expect.Equal(1, fake.PlaceCalls, "one item placed");
            fake.LandsX = 52f;
            Expect.True(!Terrain.CanStand(20f, 0f, out landed), "it lands 32 away");
            Expect.True(landed == new Vector2(52f, 0f), "nearest is where it landed");
            Expect.Near(20f, fake.AskedX, "the item was put at x");
            fake.LandsX = 11f;
            Expect.True(Terrain.CanStand(10f, 0f, out landed), "exactly 1 away is within 1");
            fake.LandsX = 11.1f;
            Expect.True(!Terrain.CanStand(10f, 0f, out landed), "more than 1 away");
        }

        /// <summary>An item that lands exactly on a cell units cannot walk (deep water) is no place to stand.</summary>
        public static void CanStandWater(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            fake.Block(112f, 80f, FakeTerrainWorld.NoWalk);
            Flat(fake, 0f, 0f);
            fake.Lands = true;
            fake.LandsX = 112f;
            fake.LandsY = 80f;
            Expect.True(!Terrain.CanStand(112f, 80f, out var landed), "the item lands on the spot, but it is not walkable");
            Expect.True(landed == new Vector2(112f, 80f), "nearest is where the item landed");
            Expect.Equal(1, fake.PlaceCalls, "the item was placed all the same");
        }

        /// <summary>LocalHeight asks the world, deformations and all, not the grid.</summary>
        public static void LocalHeight(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            fake.Ground = (x, y) => x + 2f * y;
            Expect.Near(7f, Terrain.LocalHeight(3f, 2f), "the fake's function");
            Expect.Near(0f, Terrain.Height(3f, 2f), "Height is the grid's");
            Expect.Equal(1, fake.HeightCalls, "one read of the world");
        }

        /// <summary>A read before Install throws.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            Terrain.Uninstall();
            Expect.Throws<InvalidOperationException>(() => Terrain.IsWalkable(0f, 0f), "IsWalkable");
            Expect.Throws<InvalidOperationException>(() => Terrain.IsFlyable(0f, 0f), "IsFlyable");
            Expect.Throws<InvalidOperationException>(() => Terrain.IsBuildable(0f, 0f), "IsBuildable");
            Expect.Throws<InvalidOperationException>(() => Terrain.IsWater(0f, 0f), "IsWater");
            Expect.Throws<InvalidOperationException>(() => Terrain.Refresh(new Bounds(Vector2.Zero, Vector2.One)), "Refresh");
            Expect.Throws<InvalidOperationException>(() => { Terrain.CanStand(0f, 0f, out var nearest); }, "CanStand");
            Expect.Throws<InvalidOperationException>(() => Terrain.LocalHeight(0f, 0f), "LocalHeight");
        }

        /// <summary>Installing again forgets what was read.</summary>
        public static void Reinstall(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Flat(fake, 0f, 0f);
            Expect.True(Terrain.IsWalkable(10f, 10f), "read");
            fake.Block(10f, 10f, FakeTerrainWorld.NoWalk);
            Flat(fake, 0f, 0f);
            Expect.True(!Terrain.IsWalkable(10f, 10f), "a fresh grid reads the world again");
        }
    }
}
