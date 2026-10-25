namespace Moonwell.Engine
{
    /// <summary>The package's settings with their defaults. Interactions makes one on first use, since a public static class of Moonwell sets nothing up before it is used (C17).</summary>
    internal sealed class Settings
    {
        /// <summary>The side of a grid cell, in game units.</summary>
        public float CellSize = 256.0f;

        /// <summary>How often a unit's or item's cell is read again, in seconds.</summary>
        public float CellCheck = 0.1f;

        /// <summary>How far past an interaction's range a check looks, to cover what a unit moves between two reads.</summary>
        public float Slack = 64.0f;

        /// <summary>The length of a step of the cycle, in seconds.</summary>
        public float Step = 0.03125f;
    }
}
