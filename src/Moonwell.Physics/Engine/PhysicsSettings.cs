namespace Moonwell.Engine
{
    /// <summary>The physics package's settings with their defaults. Physics makes one on first use, since a public static class of Moonwell sets nothing up before it is used (C17).</summary>
    internal sealed class PhysicsSettings
    {
        /// <summary>The downward acceleration, in game units a second squared.</summary>
        public float Gravity = 800.0f;
    }
}
