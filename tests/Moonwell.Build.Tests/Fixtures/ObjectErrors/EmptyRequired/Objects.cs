using Moonwell;

namespace EmptyRequired
{
    public sealed class EmptyRequiredMap : Map
    {
    }

    public class Nobody : Moonwell.Objects.Human.Units.Footman
    {
        public Nobody()
        {
            Name = "";
        }
    }
}
