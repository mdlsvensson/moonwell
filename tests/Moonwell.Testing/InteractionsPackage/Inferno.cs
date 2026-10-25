namespace Moonwell.Testing
{
    /// <summary>A buff that lasts a second and starts a FireAura on its unit in OnAdded, so the aura is the buff's and ends with it.</summary>
    public sealed class Inferno : Buff
    {
        /// <summary>The aura the buff starts.</summary>
        public FireAura Aura = new FireAura();

        /// <summary>Makes an inferno of a second.</summary>
        public Inferno()
        {
            Duration = 1.0f;
        }

        /// <summary>Starts the aura on the buff's unit; the buff is the current owner here.</summary>
        protected override void OnAdded()
        {
            Unit.Interact(Aura);
        }
    }
}
