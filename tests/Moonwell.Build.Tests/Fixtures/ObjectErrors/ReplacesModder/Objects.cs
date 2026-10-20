using Moonwell;

namespace ReplacesModder
{
    public sealed class ReplacesModderMap : Map
    {
    }

    public class Captain : Moonwell.Objects.Human.Units.Footman
    {
    }

    [Moonwell.Objects.Replaces]
    public class Veteran : Captain
    {
    }
}
