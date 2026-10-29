namespace Moonwell
{
    /// <summary>An ability of a unit: one object per unit and ability id, a scope that holds the casts of the ability.</summary>
    public class Ability : Scope
    {
        /// <summary>The table behind AbilityHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? abilityHooks;

        /// <summary>The reactions of abilities; declared here, so that an override in a subclass is found.</summary>
        internal static HookTable AbilityHooks
        {
            get
            {
                if (abilityHooks == null)
                {
                    abilityHooks = new HookTable(typeof(Ability), new string[] { "OnCast" }, new int[] { Hook.OnCast });
                }

                return abilityHooks;
            }
        }

        /// <summary>The bit mask of the reactions this class overrides, worked out when the ability is made for its unit.</summary>
        internal int Mask;

        /// <summary>The unit that has the ability.</summary>
        [LocalSafe]
        public Unit Unit { get; internal set; } = null!;

        /// <summary>The id of the ability.</summary>
        [LocalSafe]
        public int Id { get; internal set; }

        /// <summary>The level of the ability on its unit, as the game reads it now.</summary>
        [LocalSafe]
        public int Level => Runtime.World.AbilityLevel(Unit.Handle, Id);

        /// <summary>Runs when the unit begins a cast of the ability. This ability object is a scope that lasts as long as the caster is in the game, so a delayed effect made here with After runs after the spell has ended; work that should stop with a channel is made on the cast: cast.Every(0.5f, Pulse).</summary>
        public virtual void OnCast(Cast cast)
        {
        }
    }
}
