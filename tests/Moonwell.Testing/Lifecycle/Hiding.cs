namespace Moonwell.Testing
{
    /// <summary>A unit component whose OnDamaged hides the base's (new, not override), so it is no reaction.</summary>
    public sealed class Hiding : UnitComponent
    {
        /// <summary>How many times the hiding method ran.</summary>
        public static int Calls;

        /// <summary>Counts the call; the runtime never calls it, as it overrides nothing.</summary>
        public new void OnDamaged(Damage damage)
        {
            Calls++;
        }
    }
}
