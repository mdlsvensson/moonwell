namespace Moonwell
{
    /// <summary>A component that reacts to events of the map as a whole, such as the game's start.</summary>
    public abstract class MapComponent : Component
    {
        /// <summary>The table behind MapHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? mapHooks;

        /// <summary>The reactions of map components; declared here, so that an override in a subclass is found.</summary>
        internal static HookTable MapHooks
        {
            get
            {
                if (mapHooks == null)
                {
                    mapHooks = new HookTable(typeof(MapComponent), new[] { "OnGameStart", "OnUnitEnter", "OnUnitDamaged", "OnUnitDied", "OnUnitCast", "OnLeave" }, new[] { Hook.OnGameStart, Hook.OnUnitEnter, Hook.OnUnitDamaged, Hook.OnUnitDied, Hook.OnUnitCast, Hook.OnLeave });
                }

                return mapHooks;
            }
        }

        /// <summary>Runs when the game starts.</summary>
        public virtual void OnGameStart()
        {
        }

        /// <summary>Runs when a unit enters the game, for the game's components and then the owner's player components.</summary>
        public virtual void OnUnitEnter(Unit unit)
        {
        }

        /// <summary>Runs when a unit in the map takes damage, for the game's components, then the components of the players involved.</summary>
        public virtual void OnUnitDamaged(Damage damage)
        {
        }

        /// <summary>Runs when a unit in the map dies, for the game's components, then the components of the players involved.</summary>
        public virtual void OnUnitDied(Unit dying, Unit? killer)
        {
        }

        /// <summary>Runs when a unit in the map begins a cast, for the game's components, then the components of the caster's player.</summary>
        public virtual void OnUnitCast(Cast cast)
        {
        }

        /// <summary>Runs when a player leaves the game, for the game's components, then the leaving player's.</summary>
        public virtual void OnLeave(Player player)
        {
        }

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => MapHooks;
    }
}
