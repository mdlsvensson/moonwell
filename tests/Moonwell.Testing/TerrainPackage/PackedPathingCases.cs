using System.Text;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>A world that answers every pathing read at once and makes nothing, so a case can read Lua's memory around reads of the grid.</summary>
    public sealed class OpenGround : ITerrainWorld
    {
        /// <summary>How many times a pathing flag was read.</summary>
        public int PathCalls;

        /// <summary>The ground is flat at 0.</summary>
        public float LocationZ(float x, float y)
        {
            return 0f;
        }

        /// <summary>Units can walk everywhere.</summary>
        public bool IsWalkable(float x, float y)
        {
            PathCalls++;
            return true;
        }

        /// <summary>Flying units can fly everywhere.</summary>
        public bool IsFlyable(float x, float y)
        {
            PathCalls++;
            return true;
        }

        /// <summary>Nothing can be built anywhere.</summary>
        public bool IsBuildable(float x, float y)
        {
            PathCalls++;
            return false;
        }

        /// <summary>Ships can float everywhere.</summary>
        public bool IsFloatable(float x, float y)
        {
            PathCalls++;
            return true;
        }

        /// <summary>An item lands where it is put.</summary>
        public void Place(float x, float y, out float landedX, out float landedY)
        {
            landedX = x;
            landedY = y;
        }

        /// <summary>The playable area is the grid's middle.</summary>
        public void PlayableArea(out float minX, out float minY, out float maxX, out float maxY)
        {
            minX = 512f;
            minY = 512f;
            maxX = 3584f;
            maxY = 3584f;
        }
    }

    /// <summary>
    /// Cases from the full audit's finding F7: the pathing grid keeps a cell's four flags in four bits, eight cells to a
    /// number, so a fully read grid costs an eighth of what it did, and every cell still reads its own flags.
    /// </summary>
    public static class PackedPathingCases
    {
        /// <summary>Installs a flat grid of so many corners across and up from (0, 0) on a world.</summary>
        private static void Flat(ITerrainWorld fake, int corners)
        {
            var text = new StringBuilder();
            for (var k = 0; k < corners * corners; k++)
            {
                text.Append(GridText.Corner(0f, 2));
            }

            Terrain.Install(fake, corners, corners, 0f, 0f, text.ToString());
        }

        /// <summary>The fake's flags for a cell: each of the 16 combinations, in every place of a block's numbers.</summary>
        private static int Pattern(int column, int row)
        {
            return (column + row * 3) % 16;
        }

        /// <summary>
        /// Every cell of a full block and of a partial one (the grid is 36 cells across) reads its own four flags back, with
        /// all 16 combinations in each of the eight places of a number, the last place's highest flag included.
        /// </summary>
        public static void EveryCell(FakeWorld world)
        {
            var fake = new FakeTerrainWorld();
            for (var row = 0; row < 36; row++)
            {
                for (var column = 0; column < 36; column++)
                {
                    fake.Block(column * 32f + 16f, row * 32f + 16f, Pattern(column, row));
                }
            }

            Flat(fake, 10);
            var wrong = 0;
            var first = "";
            for (var row = 0; row < 36; row++)
            {
                for (var column = 0; column < 36; column++)
                {
                    // The fake's flags say what cannot be done; the package's what can.
                    var no = Pattern(column, row);
                    var x = column * 32f + 5f;
                    var y = row * 32f + 27f;
                    var right = Terrain.IsWalkable(x, y) == ((no & FakeTerrainWorld.NoWalk) == 0)
                        && Terrain.IsFlyable(x, y) == ((no & FakeTerrainWorld.NoFly) == 0)
                        && Terrain.IsBuildable(x, y) == ((no & FakeTerrainWorld.NoBuild) == 0)
                        && Terrain.IsWater(x, y) == ((no & FakeTerrainWorld.NoFloat) == 0);
                    if (!right)
                    {
                        wrong++;
                        if (first == "")
                        {
                            first = "cell (" + column + ", " + row + ")";
                        }
                    }
                }
            }

            Expect.Equal(0, wrong, "cells that read other flags than their own (the first: " + first + ")");
        }

        /// <summary>
        /// A grid of 128 by 128 cells read in full, 64 blocks, keeps under 100 KB of Lua's memory in the Lua host (it kept
        /// 224 KB more, a number a cell). The memory is read in the Lua host; on .NET the reads give 0.
        /// </summary>
        public static void FullGridMemory(FakeWorld world)
        {
            var fake = new OpenGround();
            Flat(fake, 33);
            Collect();
            var before = MemoryKb();
            for (var row = 0; row < 8; row++)
            {
                for (var column = 0; column < 8; column++)
                {
                    Terrain.IsWalkable(column * 512f + 256f, row * 512f + 256f);
                }
            }

            Collect();
            var kept = MemoryKb() - before;
            Expect.Equal(64 * 1024, fake.PathCalls, "64 blocks read, 256 cells at four calls");
            Expect.True(kept < 100f, "the read grid keeps " + kept + " KB");
            Expect.True(Terrain.IsWalkable(100f, 100f) && Terrain.IsFlyable(100f, 100f) && !Terrain.IsBuildable(100f, 100f) && Terrain.IsWater(100f, 100f), "a cell reads the world's flags");
        }

        // Lua's memory in KB and a full collection; the game's Lua has no collectgarbage, but these cases run in the Lua host
        // and on .NET only. The Lua host takes collectgarbage out, as the game has none, and keeps it for cases as
        // LuaHostCollectGarbage.
        /// @CSharpLua.Template = "LuaHostCollectGarbage('count')"
        private static float MemoryKb()
        {
            return 0f;
        }

        /// @CSharpLua.Template = "LuaHostCollectGarbage('collect')"
        private static void Collect()
        {
        }
    }
}
