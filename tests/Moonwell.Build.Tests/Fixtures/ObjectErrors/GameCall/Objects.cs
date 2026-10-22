using Moonwell;

namespace GameCall
{
    public sealed class GameCallMap : Map
    {
    }

    public class Lucky : Moonwell.Objects.Human.Units.Footman
    {
        public Lucky()
        {
            MaxLife = Natives.GetRandomInt(1, 2);
        }
    }
}
