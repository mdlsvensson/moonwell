#if DEBUG
using System.Collections.Generic;
#endif
namespace Moonwell
{
    /// <summary>Lines for the debug mode's panel; a release build keeps none.</summary>
    public sealed class DebugLines
    {
#if DEBUG
        /// <summary>The lines, in the order they were added.</summary>
        private readonly List<string> lines = new List<string>();

        /// <summary>How many lines have been added.</summary>
        internal int Count
        {
            get
            {
                return lines.Count;
            }
        }

        /// <summary>The lines, one to a line of text.</summary>
        internal string Text
        {
            get
            {
                var text = "";
                for (var i = 0; i < lines.Count; i++)
                {
                    text = i == 0 ? lines[i] : text + "\n" + lines[i];
                }

                return text;
            }
        }
#endif
        /// <summary>Lines are made by the debug mode only.</summary>
        internal DebugLines()
        {
        }

        /// <summary>Adds a line to the panel; a release build does nothing.</summary>
        public void Add(string line)
        {
#if DEBUG
            lines.Add(line);
#endif
        }
    }
}
