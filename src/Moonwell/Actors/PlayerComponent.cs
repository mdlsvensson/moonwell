namespace Moonwell
{
    /// <summary>A component on a player, which reacts to events of the map as a whole.</summary>
    public abstract class PlayerComponent : MapComponent
    {
        /// <summary>The player this component is on; null before the component is added, though its type does not say so (read it from OnAdded on).</summary>
        [LocalSafe]
        public Player Player => (Player)Actor;
    }
}
