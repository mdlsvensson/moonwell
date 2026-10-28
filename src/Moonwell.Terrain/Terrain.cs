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

        /// <summary>The playable area: the map's bounds less the camera's margins, as the game's own playable area is. Read once, at the start, when the package is installed and before any map code.</summary>
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
        /// cliff tiles included. It is the game's at the corners, and rough between them on steep ground and within 128
        /// of a cliff (on Lost Temple the game's heights between corners differed from such a blend by up to 77 away from
        /// cliffs and 96 near one); how the game blends inside a tile has not been measured. Deformations (spells and the
        /// like) are not in it; <c>LocalHeight</c> sees them. A point outside the map reads the edge. On a bridge it reads
        /// the water or ground beneath (the game's <c>GetLocationZ</c>, <c>LocalHeight</c>, reads the deck).
        /// </summary>
        [LocalSafe]
        public static float Height(float x, float y)
        {
            return Grid.Height(x, y);
        }

        /// <summary>The ground's height at a point, as <c>Height(x, y)</c>.</summary>
        [LocalSafe]
        public static float Height(Vector2 point)
        {
            return Height(point.X, point.Y);
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

        /// <summary>Whether a point is in a cliff tile, as <c>IsCliff(x, y)</c>.</summary>
        [LocalSafe]
        public static bool IsCliff(Vector2 point)
        {
            return IsCliff(point.X, point.Y);
        }

        /// <summary>
        /// The direction straight out of the ground at a point, a unit vector for bounces and slides, from the blended
        /// ground's slope in every tile, cliff tiles included. Outside the grid it is the slope of the edge tile.
        /// </summary>
        [LocalSafe]
        public static Vector3 Normal(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return new Vector3(-alongX, -alongY, 1f).Normalized();
        }

        /// <summary>The direction straight out of the ground at a point, as <c>Normal(x, y)</c>.</summary>
        [LocalSafe]
        public static Vector3 Normal(Vector2 point)
        {
            return Normal(point.X, point.Y);
        }

        /// <summary>
        /// How steep the ground is at a point, in degrees: 0 flat, 90 a wall. Of the blended ground in every tile, so a
        /// cliff tile reads as the slope between its corners. Outside the grid it is the slope of the edge tile.
        /// </summary>
        [LocalSafe]
        public static float Slope(float x, float y)
        {
            Grid.Gradient(x, y, out var alongX, out var alongY);
            return Maths.AngleBetween(0f, 0f, 1f, MathF.Sqrt(alongX * alongX + alongY * alongY));
        }

        /// <summary>How steep the ground is at a point, in degrees, as <c>Slope(x, y)</c>.</summary>
        [LocalSafe]
        public static float Slope(Vector2 point)
        {
            return Slope(point.X, point.Y);
        }

        /// <summary>
        /// The first place the segment from <paramref name="from"/> to <paramref name="to"/> meets the ground (the
        /// blended <c>Height</c>), with the ground's <c>Normal</c> there. Sampled every 32 along its length, so a ridge
        /// narrower than that can be stepped over. A map can move its missile to a hit's point and bounce or slide it
        /// there: a segment that starts on the ground (within 0.02 of it) hits it only where it goes under by more than
        /// 0.01 and deeper than it started, and the hit is put on the ground there; one that rises or runs along it does
        /// not hit it. A segment that starts deeper under the ground hits at fraction 0 with the normal straight up (so
        /// does one of zero length that is under it); one that stays above it gives false and an empty hit, and makes
        /// nothing. Only the part of the segment over the map's grid (the whole map, its unplayable edge included) is cast,
        /// from where it comes in over the edge: past the edge it meets nothing, so a long segment reads the ground at most
        /// about 2,700 times, across the largest map (about 25 ms in the game).
        /// </summary>
        [LocalSafe]
        public static bool Raycast(Vector3 from, Vector3 to, out TerrainHit hit)
        {
            return GroundRay.Cast(from.X, from.Y, from.Z, to.X, to.Y, to.Z, out hit);
        }

        /// <summary>
        /// Whether units can walk at a point, as the map's terrain says: cliffs, deep water and the like, not trees,
        /// buildings or units. A point outside the map reads the edge. Read a cell at a time, once, and kept until
        /// <c>Refresh</c>. Under a bridge it reads what the map's pathing file holds there: World Editor writes a placed
        /// bridge's footprint walkable, so its deck reads true; under a bridge made at runtime, or in a map whose pathing
        /// file a tool wrote, it reads the water or ground beneath (false over deep water).
        /// </summary>
        [LocalSafe]
        public static bool IsWalkable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Walkable) != 0;
        }

        /// <summary>Whether units can walk at a point, as <c>IsWalkable(x, y)</c>.</summary>
        [LocalSafe]
        public static bool IsWalkable(Vector2 point)
        {
            return IsWalkable(point.X, point.Y);
        }

        /// <summary>Whether flying units can fly at a point, as the map's terrain says. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsFlyable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Flyable) != 0;
        }

        /// <summary>Whether flying units can fly at a point, as <c>IsFlyable(x, y)</c>.</summary>
        [LocalSafe]
        public static bool IsFlyable(Vector2 point)
        {
            return IsFlyable(point.X, point.Y);
        }

        /// <summary>Whether a building can be built at a point, as the map's terrain says. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsBuildable(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Buildable) != 0;
        }

        /// <summary>Whether a building can be built at a point, as <c>IsBuildable(x, y)</c>.</summary>
        [LocalSafe]
        public static bool IsBuildable(Vector2 point)
        {
            return IsBuildable(point.X, point.Y);
        }

        /// <summary>Whether a point is water, where ships can float. A point outside the map reads the edge.</summary>
        [LocalSafe]
        public static bool IsWater(float x, float y)
        {
            return (Pathing.Flags(x, y) & PathingGrid.Water) != 0;
        }

        /// <summary>Whether a point is water, as <c>IsWater(x, y)</c>.</summary>
        [LocalSafe]
        public static bool IsWater(Vector2 point)
        {
            return IsWater(point.X, point.Y);
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

        /// <summary>Whether something can stand at a point, as <c>CanStand(x, y, out nearest)</c>; it moves an item for a moment, so it changes game state.</summary>
        [ChangesState]
        public static bool CanStand(Vector2 point, out Vector2 nearest)
        {
            return CanStand(point.X, point.Y, out nearest);
        }

        /// <summary>
        /// Whether an item put at a point lands within 1 of it: CanStand's test after IsWalkable, in floats and with no
        /// vector, for the physics package, which asks it for every pushed unit each step. Changes game state as CanStand
        /// does.
        /// </summary>
        internal static bool ItemLands(float x, float y)
        {
            World.Place(x, y, out var landedX, out var landedY);
            var dx = landedX - x;
            var dy = landedY - y;
            return dx * dx + dy * dy <= 1f;
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

        /// <summary>The ground's height at a point as this machine's game reads it, as <c>LocalHeight(x, y)</c>: only inside <c>Local.For</c>.</summary>
        [MachineLocal]
        public static float LocalHeight(Vector2 point)
        {
            return LocalHeight(point.X, point.Y);
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
