namespace Moonwell
{
    /// <summary>An ability of a unit: one object per unit and ability id, which owns the casts of the ability and what its OnCast makes.</summary>
    public class Ability : Owner
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

        /// <summary>Runs when the unit begins a cast of the ability. What it makes belongs to this ability object, which lasts as long as the caster is in the game, so a delayed effect started here runs after the spell has ended; work that should stop with a channel names the cast as its owner: Timer.Every(0.5f, Burn, cast).</summary>
        public virtual void OnCast(Cast cast)
        {
        }
    }
}
