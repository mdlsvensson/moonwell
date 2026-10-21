namespace Moonwell
{
    /// <summary>A game object that holds components and is removed from the game: a unit, an item or a destructable.</summary>
    public abstract class Widget : Actor
    {
        /// <summary>Whether the object has been removed from the game, by the runtime or found removed by the game.</summary>
        [LocalSafe]
        public bool IsGone { get; internal set; }

        /// <summary>The default object of a unit or item class, as <c>Unit.Default&lt;Champion&gt;()</c>: its declared values, the same object the Default of each of its units gives. One per class, made on first use by the class's maker, with no handle; it is gone, so its live actions do nothing. Throws for a destructable class. Declared here and not on Unit and Item, as C# allows no method named like the Default property beside it.</summary>
        [LocalSafe]
        public static T Default<T>()
            where T : Widget, new()
        {
            return (T)ObjectTypes.DefaultOf<T>();
        }
    }
}
