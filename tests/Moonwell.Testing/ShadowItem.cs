namespace Moonwell.Testing
{
    /// <summary>An item with fields of its own named as the core's fields of Item. In Lua a class's fields share one table with its base's, so each of these must stay apart from the base's field of the same name.</summary>
    public sealed class ShadowItem : Item
    {
        // A map cannot see the core's internal fields, so these hide nothing there; the tests can see them, and C# warns.
#pragma warning disable CS0108

        private int lastCharges = -7;
        private float lastX = -7.0f;
        private float lastY = -7.0f;

#pragma warning restore CS0108

        /// <summary>The names of its own fields that are no longer at their start, each followed by a space; empty when none changed.</summary>
        public string Changed()
        {
            var changed = "";
            if (lastCharges != -7) changed += "lastCharges ";
            if (lastX != -7.0f) changed += "lastX ";
            if (lastY != -7.0f) changed += "lastY ";
            return changed;
        }
    }
}
