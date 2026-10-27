namespace Moonwell.Testing
{
    /// <summary>A Separate buff of 1 s that applies itself again in OnExpire, which Moonwell refuses, as it is on its unit already; it counts its applies.</summary>
    public sealed class SelfApplying : Buff
    {
        /// <summary>How many times OnApply has run.</summary>
        public int Applied;

        /// <summary>Makes a buff of 1 s, each apply of its own.</summary>
        public SelfApplying()
        {
            Duration = 1.0f;
            Stacking = Stacking.Separate;
        }

        /// <summary>Counts the apply.</summary>
        public override void OnApply()
        {
            Applied++;
        }

        /// <summary>Applies this same buff to its unit again.</summary>
        public override void OnExpire()
        {
            Unit.Apply(this);
        }
    }
}
