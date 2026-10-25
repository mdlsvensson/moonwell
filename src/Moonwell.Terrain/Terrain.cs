using System;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>The map's ground: its height, what units can walk, fly, build and float on, and rays along it. A map's generated Main installs it with the grid the build wrote, when the map uses the package.</summary>
    public static class Terrain
    {
        /// <summary>The world the package talks to; null until Install.</summary>
        private static ITerrainWorld? world;

        /// <summary>The ground's corners; null until Install.</summary>
        private static HeightGrid? grid;

        /// <summary>The playable area, read once in Install.</summary>
        private static Bounds playable;

        /// <summary>Installs the package on the game, with the grid the build wrote: the corners across, the corners up, the position of the first corner and the corners' text.</summary>
        public static void Install(int columns, int rows, float left, float bottom, string corners)
        {
            Install(new NativeTerrainWorld(), columns, rows, left, bottom, corners);
        }

        /// <summary>Installs the package on a world; tests give a fake one. Throws ArgumentException, and changes nothing, when the text is not the length the grid needs.</summary>
        public static void Install(ITerrainWorld host, int columns, int rows, float left, float bottom, string corners)
        {
            var made = new HeightGrid(columns, rows, left, bottom, corners);
            host.PlayableArea(out var minX, out var minY, out var maxX, out var maxY);
            playable = new Bounds(new Vector2(minX, minY), new Vector2(maxX, maxY));
            world = host;
            grid = made;
        }

        /// <summary>The playable area: the map's bounds less the camera's margins, as the game's own playable area is. Read once, when the package was installed.</summary>
        [LocalSafe]
        public static Bounds Playable
        {
            get
            {
                if (world == null)
                {
                    throw NotInstalled();
                }

                return playable;
            }
        }

        /// <summary>The world the package talks to.</summary>
        internal static ITerrainWorld World
        {
            get
            {
                if (world == null)
                {
                    throw NotInstalled();
                }

                return world;
            }
        }

        /// <summary>The ground's corners.</summary>
        internal static HeightGrid Grid
        {
            get
            {
                if (grid == null)
                {
                    throw NotInstalled();
                }

                return grid;
            }
        }

        /// <summary>Forgets the install, so a case can check what reads before it do.</summary>
        internal static void Uninstall()
        {
            world = null;
            grid = null;
        }

        /// <summary>The error of a read before Install.</summary>
        private static InvalidOperationException NotInstalled()
        {
            return new InvalidOperationException("Moonwell.Terrain is not installed: the build installs it in a map that references the package");
        }
    }
}
