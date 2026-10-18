namespace Moonwell
{
    /// <summary>A component on a player, which reacts to events of the map as a whole.</summary>
    public abstract class PlayerComponent : MapComponent
    {
        /// <summary>The player this component is on.</summary>
        [LocalSafe]
        public Player Player => (Player)Actor;
    }
}
