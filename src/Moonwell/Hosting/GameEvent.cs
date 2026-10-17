namespace Moonwell.Hosting
{
    /// <summary>The game events the runtime can ask the world to deliver.</summary>
    public enum GameEvent
    {
        /// <summary>A unit takes damage.</summary>
        Damage,

        /// <summary>A unit dies.</summary>
        Death,

        /// <summary>A unit casts a spell.</summary>
        Spell,

        /// <summary>A unit picks up an item.</summary>
        PickUp,

        /// <summary>A unit drops an item.</summary>
        Drop,
    }
}
