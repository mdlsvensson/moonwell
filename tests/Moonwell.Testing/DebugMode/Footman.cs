#if DEBUG
namespace Moonwell.Testing
{
    /// <summary>A unit class for the debug mode's panel cases, with a line of its own in the panel when it is given one.</summary>
    public sealed class Footman : Unit, IDebugShown
    {
        /// <summary>The line ShowDebug adds; none when null.</summary>
        public string? Line;

        /// <summary>Adds the line, when there is one.</summary>
        public void ShowDebug(DebugLines lines)
        {
            if (Line != null)
            {
                lines.Add(Line);
            }
        }
    }
}
#endif
