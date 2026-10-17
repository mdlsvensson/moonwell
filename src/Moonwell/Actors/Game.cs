namespace Moonwell
{
    /// <summary>The game: the actor of the components that react to the map as a whole, and the root of the owners.</summary>
    public sealed class Game : Actor
    {
        /// <summary>The game of this runtime; set by Runtime.Reset.</summary>
        public static Game Instance { get; internal set; } = null!;

        /// <summary>Adds a component to the game, and returns it.</summary>
        public T Add<T>(T component) where T : GameComponent => AddComponent(component);
    }
}
