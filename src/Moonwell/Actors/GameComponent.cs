namespace Moonwell
{
    /// <summary>A component on the game, which reacts to events of the map as a whole.</summary>
    public abstract class GameComponent : MapComponent
    {
        /// <summary>The game this component is on.</summary>
        public Game Game => (Game)Actor;
    }
}
