using Moonwell;

namespace AreasFixture
{
    /// <summary>Adds the shop floor when the game starts: an area on a World Editor region is made in play, since a declared one would look the region up while the map is declared, before the package is installed.</summary>
    public sealed class Town : GameComponent
    {
        public override void OnGameStart()
        {
            Game.Instance.Add(new ShopFloor());
        }
    }
}
