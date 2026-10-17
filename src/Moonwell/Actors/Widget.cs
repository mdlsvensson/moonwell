namespace Moonwell
{
    /// <summary>A game object that holds components and is removed from the game: a unit, an item or a destructable.</summary>
    public abstract class Widget : Actor
    {
        /// <summary>Whether the object has been removed from the game, by the runtime or found removed by the game.</summary>
        public bool IsGone { get; internal set; }
    }
}
