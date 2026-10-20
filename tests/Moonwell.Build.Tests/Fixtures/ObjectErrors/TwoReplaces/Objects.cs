using Moonwell;

namespace TwoReplaces
{
    public sealed class TwoReplacesMap : Map
    {
    }

    [Moonwell.Objects.Replaces]
    public class A : Moonwell.Objects.Human.Units.Footman
    {
    }

    [Moonwell.Objects.Replaces]
    public class B : Moonwell.Objects.Human.Units.Footman
    {
    }
}
