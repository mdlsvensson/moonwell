namespace Moonwell.Testing
{
    /// <summary>A unit whose own OnDeath marks it with a buff meant to show while it is dead, as a revive timer.</summary>
    public sealed class Mourned : Unit
    {
        /// <summary>The buff OnDeath applied; null until it runs.</summary>
        public Chilled? Mark;

        /// <summary>Applies the mark.</summary>
        public override void OnDeath(Unit? killerUnit)
        {
            Mark = Apply(new Chilled(0.0f, 1.0f));
        }
    }
}
