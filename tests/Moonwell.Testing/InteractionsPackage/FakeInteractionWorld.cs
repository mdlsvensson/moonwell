using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the interactions package's cases run against: a square world, and the units, items and destructables a case marks as hidden, loaded, out of sight or dead.</summary>
    public sealed class FakeInteractionWorld : IInteractionWorld
    {
        /// <summary>The world's left edge.</summary>
        public float MinX = -1024.0f;

        /// <summary>The world's bottom edge.</summary>
        public float MinY = -1024.0f;

        /// <summary>The world's right edge.</summary>
        public float MaxX = 1024.0f;

        /// <summary>The world's top edge.</summary>
        public float MaxY = 1024.0f;

        /// <summary>How many times the package read the world's bounds.</summary>
        public int BoundsReads;

        /// <summary>How many times the package asked whether a unit is hidden, an item is visible or a destructable is alive. It asks the first of a unit alive, so this counts the reads of units, and of items and destructables.</summary>
        public int Reads;

        /// <summary>The units IsHidden answers true for.</summary>
        public List<unit> Hidden = new List<unit>();

        /// <summary>The units IsLoaded answers true for.</summary>
        public List<unit> Loaded = new List<unit>();

        /// <summary>The items IsVisible answers false for.</summary>
        public List<item> Unseen = new List<item>();

        /// <summary>The destructables IsAlive answers false for.</summary>
        public List<destructable> Dead = new List<destructable>();

        /// <summary>The destructable types IsTreeType answers true for.</summary>
        public List<int> TreeTypes = new List<int>();

        /// <summary>Whether a unit is hidden; counts the read.</summary>
        public bool IsHidden(unit target)
        {
            Reads++;
            return Hidden.Contains(target);
        }

        /// <summary>Whether a unit is loaded in a transport.</summary>
        public bool IsLoaded(unit target) => Loaded.Contains(target);

        /// <summary>Whether an item is visible; counts the read.</summary>
        public bool IsVisible(item target)
        {
            Reads++;
            return !Unseen.Contains(target);
        }

        /// <summary>Whether a destructable is alive; counts the read.</summary>
        public bool IsAlive(destructable target)
        {
            Reads++;
            return !Dead.Contains(target);
        }

        /// <summary>Whether a destructable type is a tree.</summary>
        public bool IsTreeType(int typeId) => TreeTypes.Contains(typeId);

        /// <summary>The world's bounds; counts the read.</summary>
        public void WorldBounds(out float minX, out float minY, out float maxX, out float maxY)
        {
            BoundsReads++;
            minX = MinX;
            minY = MinY;
            maxX = MaxX;
            maxY = MaxY;
        }
    }
}
