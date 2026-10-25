using System;
using System.Text;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the terrain package's install and for the grid's text: four characters a corner, unpacked a block at a time.</summary>
    public static class GridCases
    {
        /// <summary>A 2 by 2 grid whose corners are the four pinned examples: z 0 at level 2, water at -89.6, z 128 at level 3 on a ramp, and the lowest height.</summary>
        private const string Pinned4 = "YAAQXIARZQAaBgAA";

        /// <summary>The four pinned corners read back as the build wrote them.</summary>
        public static void Pinned(FakeWorld world)
        {
            var grid = new HeightGrid(2, 2, 0f, 0f, Pinned4);
            Expect.Near(0f, grid.Z(0, 0), "Z(0,0)");
            Expect.Equal(2, grid.CliffLevel(0, 0), "level (0,0)");
            Expect.True(!grid.IsRamp(0, 0) && !grid.IsWater(0, 0), "(0,0) is neither ramp nor water");
            Expect.Near(-89.6f, grid.Z(1, 0), "Z(1,0)");
            Expect.Equal(2, grid.CliffLevel(1, 0), "level (1,0)");
            Expect.True(grid.IsWater(1, 0) && !grid.IsRamp(1, 0), "(1,0) is water");
            Expect.Near(128f, grid.Z(0, 1), "Z(0,1)");
            Expect.Equal(3, grid.CliffLevel(0, 1), "level (0,1)");
            Expect.True(grid.IsRamp(0, 1) && !grid.IsWater(0, 1), "(0,1) is a ramp");
            Expect.Near(-2304f, grid.Z(1, 1), "Z(1,1)");
            Expect.Equal(0, grid.CliffLevel(1, 1), "level (1,1)");
            Expect.True(!grid.IsRamp(1, 1) && !grid.IsWater(1, 1), "(1,1) is neither ramp nor water");
        }

        /// <summary>A corner outside the grid reads as the nearest corner inside it.</summary>
        public static void Clamped(FakeWorld world)
        {
            var grid = new HeightGrid(2, 2, 0f, 0f, Pinned4);
            Expect.Near(grid.Z(0, 1), grid.Z(-1, 5), "Z(-1,5) is Z(0,1)");
            Expect.Equal(3, grid.CliffLevel(-1, 5), "level (-1,5)");
            Expect.True(grid.IsRamp(-1, 5), "(-1,5) is a ramp");
            Expect.Near(grid.Z(1, 0), grid.Z(9, -4), "Z(9,-4) is Z(1,0)");
            Expect.True(grid.IsWater(9, -4), "(9,-4) is water");
            Expect.Near(grid.Z(1, 1), grid.Z(2, 2), "Z(2,2) is Z(1,1)");
        }

        /// <summary>Text of the wrong length is refused, not read short.</summary>
        public static void WrongLength(FakeWorld world)
        {
            Expect.Throws<ArgumentException>(() => new HeightGrid(2, 2, 0f, 0f, "YAAQXIARZQAaBgA"), "15 characters");
            Expect.Throws<ArgumentException>(() => new HeightGrid(2, 2, 0f, 0f, Pinned4 + "A"), "17 characters");
            Expect.Throws<ArgumentException>(() => Terrain.Install(new FakeTerrainWorld(), 2, 2, 0f, 0f, "YAAQ"), "Install passes it on");
        }

        /// <summary>Before Install, the playable area cannot be read.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            Terrain.Uninstall();
            Expect.Throws<InvalidOperationException>(() => Terrain.Playable.ToString(), "Playable before Install");
        }

        /// <summary>The playable area is read once, in Install.</summary>
        public static void Playable(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            Terrain.Install(fake, 2, 2, 0f, 0f, Pinned4);
            var playable = Terrain.Playable;
            Expect.True(playable == new Bounds(new Vector2(-512f, -256f), new Vector2(512f, 256f)), "the area read back as Bounds");
            fake.AreaMinX = 0f;
            Expect.True(Terrain.Playable == playable, "the area is not read again");
        }

        /// <summary>Reading a corner unpacks the 16 by 16 block that holds it, once.</summary>
        public static void Blocks(FakeWorld world)
        {
            var text = new StringBuilder();
            for (var k = 0; k < 40 * 40; k++)
            {
                text.Append("YAAQ");
            }

            var grid = new HeightGrid(40, 40, -2560f, -2560f, text.ToString());
            Expect.Equal(0, grid.Unpacked, "nothing is unpacked by the constructor");
            Expect.Near(0f, grid.Z(3, 4), "Z(3,4)");
            Expect.Equal(1, grid.Unpacked, "one corner unpacked one block");
            Expect.Equal(2, grid.CliffLevel(15, 15), "level (15,15) is in the same block");
            Expect.Equal(1, grid.Unpacked, "the same block is not unpacked again");
            Expect.Near(0f, grid.Z(16, 0), "Z(16,0)");
            Expect.Equal(2, grid.Unpacked, "the next block over is the second");
            Expect.Near(0f, grid.Z(39, 39), "Z(39,39), in the last, partial block");
            Expect.Equal(3, grid.Unpacked, "the last block is the third");
            Expect.Near(0f, grid.Z(100, 100), "Z(100,100) clamps into the same block");
            Expect.Equal(3, grid.Unpacked, "a clamped read unpacks nothing new");
        }

        /// <summary>Corners in the second and third blocks, across and up, each read back as its own height.</summary>
        public static void ManyBlocks(FakeWorld world)
        {
            var text = new StringBuilder();
            for (var row = 0; row < 18; row++)
            {
                for (var column = 0; column < 20; column++)
                {
                    text.Append(GridText.Corner(column + 100 * row, 2));
                }
            }

            var grid = new HeightGrid(20, 18, 0f, 0f, text.ToString());
            Expect.Near(0f, grid.Z(0, 0), "Z(0,0), in the first block");
            Expect.Near(505f, grid.Z(5, 5), "Z(5,5), in the first block");
            Expect.Near(317f, grid.Z(17, 3), "Z(17,3), in the second block");
            Expect.Near(1702f, grid.Z(2, 17), "Z(2,17), in the third block");
            Expect.Near(1719f, grid.Z(19, 17), "Z(19,17), in the fourth block");
            Expect.Equal(4, grid.Unpacked, "four blocks hold 20 by 18 corners");
        }

        /// <summary>The digits, + and / stand for 52 to 63, and a character outside the alphabet is refused.</summary>
        public static void Alphabet(FakeWorld world)
        {
            var grid = new HeightGrid(1, 1, 0f, 0f, "+/09");
            Expect.Near(3993.3f, grid.Z(0, 0), "+/09 is 62, 63, 52, 61: h 129018");
            Expect.Equal(7, grid.CliffLevel(0, 0), "level of +/09");
            Expect.True(grid.IsWater(0, 0) && !grid.IsRamp(0, 0), "+/09 is water, no ramp");
            Expect.Throws<ArgumentException>(() => new HeightGrid(1, 1, 0f, 0f, "AA-A").Z(0, 0), "a dash");
            Expect.Throws<ArgumentException>(() => new HeightGrid(1, 1, 0f, 0f, "AA A").Z(0, 0), "a space");
            Expect.Throws<ArgumentException>(() => new HeightGrid(1, 1, 0f, 0f, "AA=A").Z(0, 0), "a padding sign");
        }
    }
}
