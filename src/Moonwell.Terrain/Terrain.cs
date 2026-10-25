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

        /// <summary>
        /// The ground's height at a point, from the map's terrain: tile corners blended, deformations (spells and the like) not in it,
        /// which <c>LocalHeight</c> sees. A point outside the map reads the edge. A tile whose corners are on different cliff levels
        /// and has no ramp steps in its middle: a point takes the level of the corner nearest it and blends only the corners on that
        /// level. On a bridge it reads the ground or water beneath, as a bridge is a destructable, not terrain.
        /// </summary>
        [LocalSafe]
        public static float Height(float x, float y)
        {
            return Grid.Height(x, y);
        }

        /// <summary>Whether a point is in a cliff tile, one whose corners are on more than one cliff level with no ramp: the face between the levels, which physics can treat as a wall.</summary>
        [LocalSafe]
        public static bool IsCliff(float x, float y)
        {
            return Grid.IsCliff(x, y);
        }

        /// <summary>The direction straight out of the ground at a point, a unit vector for bounces and slides. In a cliff tile the ground is flat at each level, so it is straight up (<c>Vector3.UnitZ</c>).</summary>
        [LocalSafe]
        public static Vector3 Normal(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return new Vector3(-alongX, -alongY, 1f).Normalized();
        }

        /// <summary>How steep the ground is at a point, in degrees: 0 flat, 90 a wall. 0 in a cliff tile, where the ground is taken as flat at each level.</summary>
        [LocalSafe]
        public static float Slope(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return Maths.AngleBetween(0f, 0f, 1f, MathF.Sqrt(alongX * alongX + alongY * alongY));
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
