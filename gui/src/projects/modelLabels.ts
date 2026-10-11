export function modelLabel(model: string): string {
  switch (model) {
    case "glm-5p3":
      return "GLM 5.3";
    case "glm-5p3-flash":
      return "GLM 5.3 Flash";
    default:
      return model;
  }
}
