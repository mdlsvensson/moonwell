namespace Moonwell.Testing
{
    /// <summary>A unit that starts a FireAura on itself in its OnCreated, before the runtime tells the interactions package it was taken in.</summary>
    public sealed class AuraBearer : Unit
    {
        /// <summary>The aura it starts.</summary>
        public FireAura Aura = new FireAura();

        /// <summary>Starts the aura on the unit, in the unit's scope.</summary>
        protected override void OnCreated()
        {
            this.Interact(Aura, this);
        }
    }
}
