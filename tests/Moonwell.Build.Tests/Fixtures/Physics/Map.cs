using Moonwell;

namespace PhysicsFixture
{
    /// <summary>A map that uses the physics package: two footmen made at the start, and a moment later a firebolt fired from one at the other and a knockback.</summary>
    public sealed class PhysicsMap : Map
    {
        public PhysicsMap()
        {
            Name = "Physics";
            Components.Add(new Launcher());
        }
    }
}
