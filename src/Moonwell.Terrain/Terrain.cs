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

        /// <summary>The pathing flags of the ground, read a block at a time; null until Install.</summary>
        private static PathingGrid? pathing;

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
            pathing = new PathingGrid(host, columns, rows, left, bottom);
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
        /// The ground's height at a point, from the map's terrain: the four corners of its tile blended, in every tile,
        /// cliff tiles included. Deformations (spells and the like) are not in it; <c>LocalHeight</c> sees them. A
        /// point outside the map reads the edge. On a bridge it reads the water or ground beneath (the game's
        /// <c>GetLocationZ</c>, <c>LocalHeight</c>, reads the deck).
        /// </summary>
        [LocalSafe]
        public static float Height(float x, float y)
        {
            return Grid.Height(x, y);
        }

        /// <summary>
        /// Whether a point is in a cliff tile, one whose corners are on more than one cliff level with no ramp: the
        /// face between the levels, which physics can treat as a wall. <c>Height</c> still blends there.
        /// </summary>
        [LocalSafe]
        public static bool IsCliff(float x, float y)
        {
            return Grid.IsCliff(x, y);
        }

        /// <summary>
        /// The direction straight out of the ground at a point, a unit vector for bounces and slides, from the blended
        /// ground's slope in every tile, cliff tiles included.
        /// </summary>
        [LocalSafe]
        public static Vector3 Normal(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return new Vector3(-alongX, -alongY, 1f).Normalized();
        }

        /// <summary>
        /// How steep the ground is at a point, in degrees: 0 flat, 90 a wall. Of the blended ground in every tile, so a
        /// cliff tile reads as the slope between its corners.
        /// </summary>
        [LocalSafe]
        public static float Slope(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return Maths.AngleBetween(0f, 0f, 1f, MathF.Sqrt(alongX * alongX + alongY * alongY));
        }

        /// <summary>
        /// Whether units can walk at a point, as the map's terrain says: cliffs, deep water and the like, not trees,
        /// buildings or units. A point outside the map reads the edge. Read a cell at a time, once, and kept until
        /// <c>Refresh</c>. On a bridge it reads false, as the water beneath does.
        /// </summary>
        [LocalSafe]
        public static bool IsWalkable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Walkable) != 0;
        }

        /// <summary>Whether flying units can fly at a point, as the map's terrain says. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsFlyable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Flyable) != 0;
        }

        /// <summary>Whether a building can be built at a point, as the map's terrain says. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsBuildable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Buildable) != 0;
        }

        /// <summary>Whether a point is water, where ships can float. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsWater(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Water) != 0;
        }

        /// <summary>
        /// Forgets the pathing read for the blocks of cells an area touches, so the next read of a point in them asks
        /// the game again. Call it after a map changes the pathing with natives.
        /// </summary>
        [ChangesState]
        public static void Refresh(Bounds area)
        {
            Pathing.Refresh(area);
        }

        /// <summary>
        /// Whether something can stand at a point: the pathing grid says the point is walkable (<c>IsWalkable</c>) and
        /// an item put there lands within 1 of it, which trees, buildings and unwalkable ground push it away from. Both
        /// are needed, as the game lays the item exactly on deep water. <c>nearest</c> is where the item landed, always
        /// set, as the item is put down either way. This changes game state, as the item is moved for a moment. On a
        /// bridge it reads false, as the item will not lie on one.
        /// </summary>
        [ChangesState]
        public static bool CanStand(float x, float y, out Vector2 nearest)
        {
            World.Place(x, y, out var landedX, out var landedY);
            nearest = new Vector2(landedX, landedY);
            var dx = landedX - x;
            var dy = landedY - y;
            return IsWalkable(x, y) && dx * dx + dy * dy <= 1f;
        }

        /// <summary>
        /// The ground's height at a point as this machine's game reads it, deformations included. The game can answer
        /// differently on each machine, so a map reads it only inside <c>Local.For</c>.
        /// </summary>
        [MachineLocal]
        public static float LocalHeight(float x, float y)
        {
            return World.LocationZ(x, y);
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

        /// <summary>The pathing flags of the ground.</summary>
        internal static PathingGrid Pathing
        {
            get
            {
                if (pathing == null)
                {
                    throw NotInstalled();
                }

                return pathing;
            }
        }

        /// <summary>Forgets the install, so a case can check what reads before it do.</summary>
        internal static void Uninstall()
        {
            world = null;
            grid = null;
            pathing = null;
        }

        /// <summary>The error of a read before Install.</summary>
        private static InvalidOperationException NotInstalled()
        {
            return new InvalidOperationException("Moonwell.Terrain is not installed: the build installs it in a map that references the package");
        }
    }
}
