using Moonwell;

namespace Fails
{
    /// <summary>A map whose components fail in every way the error lines name.</summary>
    public sealed class FailsMap : Map
    {
        public FailsMap()
        {
            Name = "Fails";
            Components.Add(new Throws());
            Components.Add(new ReadsNull());
            Components.Add(new CallsHelper());
            Components.Add(new TimerThrows());
            Components.Add(new ReadsProperty());
            Components.Add(new EveryZero());
        }
    }
}
