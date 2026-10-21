namespace Moonwell
{
    /// <summary>The game: the actor of the components that react to the map as a whole, and the root of the owners.</summary>
    public sealed class Game : Actor
    {
        /// <summary>The game of this runtime; set by Runtime.Reset.</summary>
        [LocalSafe]
        public static Game Instance { get; internal set; } = null!;

        /// <summary>Shows a line to every player, for the seconds, or for the game's own time without them; it changes nothing in the game.</summary>
        [LocalSafe]
        public static void Print(string text, float seconds = -1.0f)
        {
            Runtime.World.PrintToAll(text, seconds);
        }

        /// <summary>Adds a component to the game, and returns it.</summary>
        public T Add<T>(T component) where T : GameComponent => AddComponent(component);
    }
}
