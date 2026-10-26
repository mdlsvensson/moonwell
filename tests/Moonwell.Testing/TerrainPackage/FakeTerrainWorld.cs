using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the terrain package's cases run against: ground height from a function, pathing flags per 32 by 32 cell, and an item that lands where the case says. It counts its calls.</summary>
    public sealed class FakeTerrainWorld : ITerrainWorld
    {
        /// <summary>The flag of a cell where units cannot walk.</summary>
        public const int NoWalk = 1;

        /// <summary>The flag of a cell where flying units cannot fly.</summary>
        public const int NoFly = 2;

        /// <summary>The flag of a cell where nothing can be built.</summary>
        public const int NoBuild = 4;

        /// <summary>The flag of a cell where ships cannot float.</summary>
        public const int NoFloat = 8;

        /// <summary>The flags of the cells that have any; a cell not in it allows everything.</summary>
        private readonly Dictionary<string, int> cells = new Dictionary<string, int>();

        /// <summary>The ground's height at a point; flat at 0 until a case sets it.</summary>
        public Func<float, float, float> Ground = (x, y) => 0f;

        /// <summary>How many times the height was read.</summary>
        public int HeightCalls;

        /// <summary>How many times a pathing flag was read.</summary>
        public int PathCalls;

        /// <summary>How many times an item was placed.</summary>
        public int PlaceCalls;

        /// <summary>The x of the last place the package asked for.</summary>
        public float AskedX;

        /// <summary>The y of the last place the package asked for.</summary>
        public float AskedY;

        /// <summary>Whether the item lands at LandsX, LandsY instead of where it was put.</summary>
        public bool Lands;

        /// <summary>The x the item lands at when Lands is true.</summary>
        public float LandsX;

        /// <summary>The y the item lands at when Lands is true.</summary>
        public float LandsY;

        /// <summary>The playable area's lower-left x.</summary>
        public float AreaMinX = -512f;

        /// <summary>The playable area's lower-left y.</summary>
        public float AreaMinY = -256f;

        /// <summary>The playable area's upper-right x.</summary>
        public float AreaMaxX = 512f;

        /// <summary>The playable area's upper-right y.</summary>
        public float AreaMaxY = 256f;

        /// <summary>Sets the flags of the 32 by 32 cell that holds a point.</summary>
        public void Block(float x, float y, int flags)
        {
            cells[Key(x, y)] = flags;
        }

        /// <summary>The ground's height at a point.</summary>
        public float LocationZ(float x, float y)
        {
            HeightCalls++;
            return Ground(x, y);
        }

        /// <summary>Whether units can walk at a point.</summary>
        public bool IsWalkable(float x, float y) => Allowed(x, y, NoWalk);

        /// <summary>Whether flying units can fly at a point.</summary>
        public bool IsFlyable(float x, float y) => Allowed(x, y, NoFly);

        /// <summary>Whether a building can be built at a point.</summary>
        public bool IsBuildable(float x, float y) => Allowed(x, y, NoBuild);

        /// <summary>Whether ships can float at a point.</summary>
        public bool IsFloatable(float x, float y) => Allowed(x, y, NoFloat);

        /// <summary>Puts the item down: where it was asked, or where the case says it lands.</summary>
        public void Place(float x, float y, out float landedX, out float landedY)
        {
            PlaceCalls++;
            AskedX = x;
            AskedY = y;
            landedX = Lands ? LandsX : x;
            landedY = Lands ? LandsY : y;
        }

        /// <summary>The playable area the case set.</summary>
        public void PlayableArea(out float minX, out float minY, out float maxX, out float maxY)
        {
            minX = AreaMinX;
            minY = AreaMinY;
            maxX = AreaMaxX;
            maxY = AreaMaxY;
        }

        private bool Allowed(float x, float y, int flag)
        {
            PathCalls++;
            return !cells.TryGetValue(Key(x, y), out var flags) || (flags & flag) == 0;
        }

        private static string Key(float x, float y)
        {
            return (int)MathF.Floor(x / 32f) + "," + (int)MathF.Floor(y / 32f);
        }
    }
}
