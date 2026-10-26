namespace Moonwell
{
    /// <summary>IsTree on a destructable.</summary>
    public static class TreeExtensions
    {
        /// <summary>Whether the destructable is a tree: its type's "Targeted As" holds tree, which the build wrote into the map's object Lua. False in a map built without that table (one with no <c>Moonwell.Objects</c>). A read of a table; it makes nothing and changes nothing.</summary>
        [LocalSafe]
        public static bool IsTree(this Destructable destructable)
        {
            return Interactions.PeekWorld.IsTreeType(destructable.TypeId);
        }
    }
}
