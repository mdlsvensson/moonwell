namespace Moonwell
{
    /// <summary>A component on the game, which reacts to events of the map as a whole.</summary>
    public abstract class GameComponent : MapComponent
    {
        /// <summary>The game this component is on; null before the component is added, though its type does not say so (read it from OnAdded on).</summary>
        [LocalSafe]
        public Game Game => (Game)Actor;
    }
}
