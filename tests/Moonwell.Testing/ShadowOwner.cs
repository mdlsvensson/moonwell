namespace Moonwell.Testing
{
    /// <summary>An owner with a field of its own named as Owner's count of children. In Lua a class's fields share one table with its base's, so this one must stay apart from the base's.</summary>
    public sealed class ShadowOwner : Owner
    {
        // A map cannot see the core's internal fields, so this hides nothing there; the tests can see it, and C# warns.
#pragma warning disable CS0108

        /// <summary>Its own count, 7; the owner's OwnedCount counts its children.</summary>
        private int count = 7;

#pragma warning restore CS0108

        /// <summary>Its own count.</summary>
        public int OwnCount
        {
            get { return count; }
        }
    }
}
