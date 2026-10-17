namespace Moonwell
{
    /// <summary>A component that reacts to events of the map as a whole, such as the game's start.</summary>
    public abstract class MapComponent : Component
    {
        /// <summary>The reactions of map components; declared here, so that an override in a subclass is found.</summary>
        internal static readonly HookTable MapHooks = new HookTable(typeof(MapComponent), new[] { "OnGameStart", "OnUnitEnter" }, new[] { Hook.OnGameStart, Hook.OnUnitEnter });

        /// <summary>Runs when the game starts.</summary>
        public virtual void OnGameStart()
        {
        }

        /// <summary>Runs when a unit enters the game, for the game's components and then the owner's player components.</summary>
        public virtual void OnUnitEnter(Unit unit)
        {
        }

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => MapHooks;
    }
}
