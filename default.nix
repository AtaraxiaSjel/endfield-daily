{ lib, buildGoModule }:
buildGoModule {
  pname = "endfield-daily";
  version = "1.0.0";
  src = lib.cleanSource ./.;

  vendorHash = "sha256-2U9hBA+q3nW4YO47PN+eorBLq0z4kj2zCZ6q7wD75PQ=";

  meta = with lib; {
    description = "Daily check-in bot for Arknights: Endfield";
    license = licenses.mit;
    maintainers = with maintainers; [ ataraxiasjel ];
    mainProgram = "endfield-daily";
  };
}
