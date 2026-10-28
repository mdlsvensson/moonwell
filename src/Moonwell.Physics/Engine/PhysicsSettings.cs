using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>The physics package's settings with their defaults. Physics makes one on first use, since a public static class of Moonwell sets nothing up before it is used (C17).</summary>
    internal sealed class PhysicsSettings
    {
        /// <summary>The downward acceleration, in game units a second squared.</summary>
        public float Gravity = 800.0f;

        /// <summary>The radius DestructableSize gave each destructable type, by type id; null until the first. A lookup only.</summary>
        public Dictionary<int, float>? DestructableRadii;

        /// <summary>The height DestructableSize gave each destructable type, by type id; null until the first. A lookup only.</summary>
        public Dictionary<int, float>? DestructableHeights;

        /// <summary>The largest radius DestructableSize has given; 0 before one.</summary>
        public float LargestDestructable;
    }
}
