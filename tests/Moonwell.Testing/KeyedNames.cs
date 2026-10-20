using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Names kept per unit in a static dictionary keyed by the unit object, as a map writes it.</summary>
    public static class KeyedNames
    {
        /// <summary>The name of each unit; its type is made while the script loads, which runs before Unit's file is set up when this file comes first.</summary>
        public static readonly Dictionary<Unit, string> Names = new Dictionary<Unit, string>();
    }
}
