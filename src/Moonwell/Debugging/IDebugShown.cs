namespace Moonwell
{
    /// <summary>A map's class or component with its own lines in the debug mode's panel while its object is selected; a release build never calls it.</summary>
    public interface IDebugShown
    {
        /// <summary>Adds the object's lines to the panel, one Add each.</summary>
        void ShowDebug(DebugLines lines);
    }
}
