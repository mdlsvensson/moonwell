using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>The lines the unit test types write, which each case clears before it runs.</summary>
    public static class TestLog
    {
        /// <summary>The lines written so far, in order.</summary>
        public static List<string> Lines = new List<string>();
    }
}
